package notifier

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	types "mfcbot/internal"
	"mfcbot/internal/database"
)

// sender is the Discord side of the notifier; *discord.Client satisfies it.
type sender interface {
	SendMessage(channelID, content string) error
	SendMessageWithFile(channelID, content, filename string, data []byte) error
}

// Notifier periodically compares the watched models in the database against
// the live MFC online snapshot and posts Discord notifications on changes.
type Notifier struct {
	db       *database.Database
	discord  sender
	online   func() map[string]types.StreamerInfo
	ready    func() bool
	interval time.Duration
	client   *http.Client
	trigger  chan struct{}
}

// New creates a new Notifier. ready may be nil; when non-nil it gates checks
// so a snapshot that hasn't received its initial bulk yet is never mistaken
// for "every model is offline".
func New(db *database.Database, dc sender, online func() map[string]types.StreamerInfo, ready func() bool, interval time.Duration) *Notifier {
	return &Notifier{
		db:       db,
		discord:  dc,
		online:   online,
		ready:    ready,
		interval: interval,
		client:   &http.Client{Timeout: 30 * time.Second},
		trigger:  make(chan struct{}, 1),
	}
}

// Trigger requests an immediate online check (used after addmodel, matching
// the original bot).
func (n *Notifier) Trigger() {
	select {
	case n.trigger <- struct{}{}:
	default:
	}
}

// Run resets topic state at startup (matching the original bot) and then
// checks online status on a fixed interval until ctx is cancelled.
func (n *Notifier) Run(ctx context.Context) {
	n.resetTopics()
	// Wait for the first MFC bulk dump before judging any model offline.
	for n.ready != nil && !n.ready() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
	n.check(ctx)

	t := time.NewTicker(n.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n.check(ctx)
		case <-n.trigger:
			n.check(ctx)
		}
	}
}

// resetTopics clears the stored topic for every model so the first check
// re-announces currently-online models, matching the original bot's startup.
func (n *Notifier) resetTopics() {
	models, err := n.db.GetModels()
	if err != nil {
		log.Printf("notifier: get models: %v", err)
		return
	}
	for _, m := range models {
		if _, err := n.db.UpdateTopic(m.ModelID, "", m.Channel); err != nil {
			log.Printf("notifier: reset topic: %v", err)
		}
	}
}

// check performs one full pass over all watched models.
func (n *Notifier) check(ctx context.Context) {
	if n.ready != nil && !n.ready() {
		return
	}
	models, err := n.db.GetModels()
	if err != nil {
		log.Printf("notifier: get models: %v", err)
		return
	}

	// Index the live snapshot by MFC uid, which is what the DB stores as modelid.
	byUID := make(map[int]types.StreamerInfo)
	for _, info := range n.online() {
		byUID[info.UID] = info
	}

	for _, m := range models {
		if ctx.Err() != nil {
			return
		}
		uid, err := strconv.Atoi(m.ModelID)
		if err != nil {
			continue
		}
		info, online := byUID[uid]
		if online {
			n.handleOnline(m, info)
		} else {
			n.handleOffline(m)
		}
	}
}

// handleOnline posts notifications for a model that is currently online.
func (n *Notifier) handleOnline(m database.Model, info types.StreamerInfo) {
	switch info.ShowKind {
	case types.ShowGroup:
		if m.Topic != "2" {
			n.send(m, m.ModelName+" is now in a group show!")
			n.updateTopic(m, "2")
		}
	case types.ShowTicket:
		if m.Topic != "3" {
			n.send(m, m.ModelName+" is now in a club show!")
			n.updateTopic(m, "3")
		}
	case types.ShowPublic:
		if m.Topic != "Public" {
			// The original bot only announces public shows that have a topic.
			if info.Subject == "" {
				return
			}
			message := m.Message + "\nCurrent topic is:\n" + info.Subject
			n.sendWithImage(m, message, info.ImageURL)
			n.updateTopic(m, "Public")
			n.updateTime(m, time.Now().UnixMilli())
		}
	}
}

// handleOffline posts notifications for a model that is no longer online.
func (n *Notifier) handleOffline(m database.Model) {
	switch {
	case m.Topic != "":
		// Was online during this session; report the offline duration.
		hours, minutes, seconds := onlineDuration(m.Time)
		message := fmt.Sprintf(
			"%s has gone offline. She was online for %d hours %d minutes and %d seconds",
			m.ModelName, hours, minutes, seconds,
		)
		n.send(m, message)
		n.updateTime(m, 0)
		n.updateTopic(m, "")
	case m.Time != 0:
		// Went offline while the bot itself was down.
		n.send(m, m.ModelName+" has gone offline while the bot was offline.")
		n.updateTime(m, 0)
		n.updateTopic(m, "")
	}
}

// onlineDuration reproduces the original bot's
// `new Date(Date.now() - time).getUTC*` calculation.
func onlineDuration(storedMillis int64) (hours, minutes, seconds int) {
	elapsed := time.Now().UnixMilli() - storedMillis
	t := time.UnixMilli(elapsed).UTC()
	return t.Hour(), t.Minute(), t.Second()
}

func (n *Notifier) updateTopic(m database.Model, topic string) {
	if _, err := n.db.UpdateTopic(m.ModelID, topic, m.Channel); err != nil {
		log.Printf("notifier: update topic for %s: %v", m.ModelID, err)
	}
}

func (n *Notifier) updateTime(m database.Model, millis int64) {
	if _, err := n.db.UpdateTime(m.ModelID, millis, m.Channel); err != nil {
		log.Printf("notifier: update time for %s: %v", m.ModelID, err)
	}
}

func (n *Notifier) send(m database.Model, content string) {
	if m.Channel == "" {
		return
	}
	if err := n.discord.SendMessage(m.Channel, content); err != nil {
		log.Printf("notifier: %v", err)
	}
}

func (n *Notifier) sendWithImage(m database.Model, content, imageURL string) {
	if m.Channel == "" {
		return
	}
	data, err := n.download(imageURL)
	if err != nil || len(data) == 0 {
		// Fall back to a text-only message so the notification isn't lost.
		log.Printf("notifier: image download failed for %s: %v", m.ModelID, err)
		n.send(m, content)
		return
	}
	filename := fmt.Sprintf("%s.jpg", m.ModelID)
	if err := n.discord.SendMessageWithFile(m.Channel, content, filename, data); err != nil {
		log.Printf("notifier: %v", err)
	}
}

func (n *Notifier) download(url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty url")
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
}
