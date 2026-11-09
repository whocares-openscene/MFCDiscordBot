// In-memory online-models state for the MFC adapter, plus the methods that
// mutate and read it.
package mfc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	types "mfcbot/internal"
	"mfcbot/internal/config"
	"mfcbot/internal/logging"
)

// streamer is one online model's last known state, keyed by uid in the snapshot.
type streamer struct {
	name    string
	uid     int
	vs      int
	rc      int
	topic   string
	camserv int
}

// nameEntry is the per-uid state we keep in nameCache.
type nameEntry struct {
	name     string
	lastSeen time.Time
	lookupAt time.Time
}

// onlineState is the in-memory online-models state, updated by the reader and
// read by the HTTP handlers.
type onlineState struct {
	mu                           sync.RWMutex
	online                       map[int]*streamer
	nameCache                    map[int]nameEntry
	videoHosts                   map[int]string
	bulkApplied                  bool
	lifetimeDisconnects          atomic.Int32
	lifetimeServerConfigFailures atomic.Int32
	currentSession               atomic.Pointer[wsSession]
}

func newOnlineState() *onlineState {
	return &onlineState{
		online:     map[int]*streamer{},
		nameCache:  map[int]nameEntry{},
		videoHosts: map[int]string{},
	}
}

// setVideoHosts atomically replaces the camserv → video host map.
func (s *onlineState) setVideoHosts(hosts map[int]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videoHosts = hosts
}

// rememberName records uid → name with the current timestamp.
func (s *onlineState) rememberName(uid int, name string) {
	if name == "" {
		return
	}
	s.nameCache[uid] = nameEntry{name: name, lastSeen: time.Now()}
}

// validate returns a non-nil error if the state is inconsistent.
func (s *onlineState) validate() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.bulkApplied && len(s.online) == 0 {
		return errors.New("post-bulk snapshot empty, MFC data inconsistent")
	}
	return nil
}

// replaceFromBulk replaces the state with a fresh bulk dump.
func (s *onlineState) replaceFromBulk(rows bulk, lookupTTL time.Duration) (toLookup []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.online = make(map[int]*streamer, len(rows))
	for _, r := range rows {
		if r.vs == mfcFCVideoOffline {
			logging.Ldbg("bulk contains offline row: @uid = %d, @nm = %s", r.uid, r.name)
			continue
		}
		name := r.name
		if name == "" {
			if cached, found := s.nameCache[r.uid]; found && cached.name != "" {
				name = cached.name
			}
		}
		s.online[r.uid] = &streamer{
			name:    name,
			uid:     r.uid,
			vs:      r.vs,
			rc:      r.rc,
			topic:   decodeMFCTopic(r.topic),
			camserv: r.camserv,
		}
		if name == "" {
			if s.claimLookupLocked(r.uid, lookupTTL) {
				toLookup = append(toLookup, r.uid)
			}
		} else {
			s.rememberName(r.uid, name)
		}
	}
	s.bulkApplied = true
	return toLookup
}

// applyUpdate merges one SESSIONSTATE update.
func (s *onlineState) applyUpdate(u sessionUpdate, cfg *config.Config) error {
	if u.UID == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	uid := *u.UID
	if u.VS != nil && *u.VS == mfcFCVideoOffline {
		s.handleOffline(u)
		return nil
	}
	cur, existed := s.online[uid]
	if !existed {
		if len(s.online) >= cfg.MaxSnapshotSize {
			return fmt.Errorf("snapshot exceeded %d entries", cfg.MaxSnapshotSize)
		}
		cur = &streamer{uid: uid}
		s.online[uid] = cur
	}
	if u.Name != nil {
		cur.name = *u.Name
	}
	if cur.name == "" {
		if cached, found := s.nameCache[uid]; found && cached.name != "" {
			cur.name = cached.name
		}
	}
	if u.VS != nil {
		cur.vs = *u.VS
	}
	if rc := u.rc(); rc != nil {
		cur.rc = *rc
	}
	if topic := u.topic(); topic != nil {
		cur.topic = decodeMFCTopic(*topic)
	}
	if cs := u.camserv(); cs != nil {
		cur.camserv = *cs
	}
	if cur.name != "" {
		s.rememberName(uid, cur.name)
	}
	return nil
}

// handleOffline marks uid offline.
func (s *onlineState) handleOffline(u sessionUpdate) {
	uid := *u.UID
	name := ""
	switch {
	case u.Name != nil && *u.Name != "":
		name = *u.Name
	default:
		if cur, ok := s.online[uid]; ok && cur.name != "" {
			name = cur.name
		} else if cached, found := s.nameCache[uid]; found {
			name = cached.name
		}
	}
	if name != "" {
		s.rememberName(uid, name)
	}
	delete(s.online, uid)
}

// markDisconnected wipes the online map and clears bulkApplied.
func (s *onlineState) markDisconnected() (hadBulkApplied bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hadBulkApplied = s.bulkApplied
	clear(s.online)
	s.bulkApplied = false
	for uid, e := range s.nameCache {
		if e.name == "" {
			delete(s.nameCache, uid)
		}
	}
	return hadBulkApplied
}

// applyRoomData updates room counts in bulk from a ROOMDATA frame.
func (s *onlineState) applyRoomData(counts map[int]int) (matched, unknown int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uid, rc := range counts {
		if cur, ok := s.online[uid]; ok {
			cur.rc = rc
			matched++
		} else {
			unknown++
		}
	}
	return matched, unknown
}

// claimLookup reports whether uid is online without a name and its previous
// USERNAMELOOKUP claim is older than ttl.
func (s *onlineState) claimLookup(uid int, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.online[uid]
	if !ok || cur.name != "" {
		return false
	}
	return s.claimLookupLocked(uid, ttl)
}

// claimLookupLocked is the cache-side core of claimLookup.
func (s *onlineState) claimLookupLocked(uid int, ttl time.Duration) bool {
	e := s.nameCache[uid]
	if e.name != "" {
		return false
	}
	if !e.lookupAt.IsZero() && time.Since(e.lookupAt) < ttl {
		return false
	}
	now := time.Now()
	e.lookupAt = now
	if e.lastSeen.IsZero() {
		e.lastSeen = now
	}
	s.nameCache[uid] = e
	return true
}

// pruneNameCache drops nameCache entries older than the applicable TTL.
func (s *onlineState) pruneNameCache(resolvedTTL, pendingTTL time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	resolvedCutoff := now.Add(-resolvedTTL)
	pendingCutoff := now.Add(-pendingTTL)
	removed := 0
	for uid, e := range s.nameCache {
		cutoff := resolvedCutoff
		if e.name == "" {
			cutoff = pendingCutoff
		}
		if e.lastSeen.Before(cutoff) {
			delete(s.nameCache, uid)
			removed++
		}
	}
	return removed
}

// collectIfReady returns the streamers map and a readiness flag.
func (s *onlineState) collectIfReady() (streamers map[string]types.StreamerInfo, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.bulkApplied {
		return nil, false
	}
	out := make(map[string]types.StreamerInfo, len(s.online))
	for _, cur := range s.online {
		if cur.name == "" {
			continue
		}
		viewers := cur.rc
		img := mfcLiveSnapshotURL(cur.uid, cur.vs, s.videoHosts[cur.camserv])
		if img == "" {
			img = mfcImageURL(cur.uid)
		}
		out[strings.ToLower(cur.name)] = types.StreamerInfo{
			ImageURL: img,
			Viewers:  &viewers,
			ShowKind: mfcShowKind(cur.vs),
			Subject:  cur.topic,
			UID:      cur.uid,
		}
	}
	return out, true
}

// bulkReady reports whether a bulk dump has been applied in the current session.
func (s *onlineState) bulkReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bulkApplied
}

// applyBulk fetches the deferred MANAGELIST/CAMS payload referenced by ext
// and replaces the state with it.
func (s *onlineState) applyBulk(
	ctx context.Context,
	cfg *config.Config,
	client *http.Client,
	ext *mfcExtData,
) ([]int, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, cfg.ExtDataFetchTimeout)
	defer cancel()
	rows, err := fetchExtData(fetchCtx, client, cfg, ext)
	if err != nil {
		return nil, err
	}
	if err := rows.validate(cfg); err != nil {
		return nil, err
	}
	toLookup := s.replaceFromBulk(rows, cfg.LookupTTL)
	logging.Linf("bulk dump applied: %s", rows.breakdownByShowKind())
	return toLookup, nil
}

// validate reports whether a freshly fetched bulk is fit to install.
func (b bulk) validate(cfg *config.Config) error {
	switch {
	case len(b) == 0:
		return errors.New("empty bulk")
	case len(b) > cfg.MaxSnapshotSize:
		return fmt.Errorf("bulk exceeded %d entries", cfg.MaxSnapshotSize)
	}
	return nil
}

// breakdownByShowKind formats a per-show-kind count summary for a bulk dump.
func (b bulk) breakdownByShowKind() string {
	var public, private, group, club, away, other, offline int
	for _, r := range b {
		if r.vs == mfcFCVideoOffline {
			offline++
			continue
		}
		switch mfcShowKind(r.vs) {
		case types.ShowPublic:
			public++
		case types.ShowPrivate:
			private++
		case types.ShowGroup:
			group++
		case types.ShowTicket:
			club++
		case types.ShowAway:
			away++
		default:
			other++
		}
	}
	total := public + private + group + club + away + other
	parts := []string{fmt.Sprintf("%d total", total)}
	for _, p := range []struct {
		label string
		n     int
	}{
		{"public", public}, {"private", private}, {"group", group},
		{"club", club}, {"away", away}, {"other", other}, {"offline", offline},
	} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", p.label, p.n))
		}
	}
	return strings.Join(parts, ", ")
}

// mfcLiveSnapshotURL builds a 853x480 live snapshot URL.
func mfcLiveSnapshotURL(uid, vs int, videoHost string) string {
	if videoHost == "" {
		return ""
	}
	var snapshotPrefix string
	switch vs {
	case 0, 90:
	case 2:
		snapshotPrefix = "s_"
	default:
		return ""
	}
	return fmt.Sprintf("https://%s.myfreecams.com/snaps/mfc_%s%d_853x480.jpg?no-cache=%d",
		videoHost, snapshotPrefix, uid, time.Now().UnixNano())
}

// mfcImageURL returns the avatar URL for a uid.
func mfcImageURL(uid int) string {
	s := fmt.Sprintf("%d", uid)
	return fmt.Sprintf(
		"https://img.mfcimg.com/photos2/%s/%s/avatar.300x300.jpg",
		s[:min(3, len(s))], s)
}
