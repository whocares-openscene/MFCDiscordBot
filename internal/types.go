package types

import (
	"fmt"
	"time"
)

// ShowKind represents the kind of show a model is currently in
type ShowKind int

const (
	ShowUnknown ShowKind = iota
	ShowPublic
	ShowGroup
	ShowTicket
	ShowHidden
	ShowPrivate
	ShowAway
)

func (s ShowKind) String() string {
	switch s {
	case ShowUnknown:
		return "unknown"
	case ShowPublic:
		return "public"
	case ShowGroup:
		return "group"
	case ShowTicket:
		return "ticket"
	case ShowHidden:
		return "hidden"
	case ShowPrivate:
		return "private"
	case ShowAway:
		return "away"
	}
	return fmt.Sprintf("ShowKind(%d)", int(s))
}

// StreamerInfo carries presentation data for a streamer
type StreamerInfo struct {
	ImageURL string   `json:"image_url,omitempty"`
	Viewers  *int     `json:"viewers,omitempty"`
	ShowKind ShowKind `json:"show_kind,omitempty"`
	Subject  string   `json:"subject,omitempty"`
	UID      int      `json:"uid,omitempty"`
}

// StreamerInfoWithStatus is StreamerInfo plus a StatusKind verdict
type StreamerInfoWithStatus struct {
	StreamerInfo
	Status StatusKind `json:"status"`
}

// StatusKind represents a status of a streamer
type StatusKind int

const (
	StatusUnknown StatusKind = iota
	StatusOffline
	StatusOnline
	StatusNotFound
	StatusDenied
)

// OnlineListResults contains results for the /online endpoint
type OnlineListResults struct {
	Streamers  map[string]StreamerInfo `json:"streamers"`
	PollCount  int                     `json:"poll_count,omitempty"`
	PollErrors []string                `json:"poll_errors,omitempty"`
	Duration   time.Duration           `json:"duration"`
	Failed     bool                    `json:"failed"`
}

// NewOnlineListResults creates a successful OnlineListResults
func NewOnlineListResults(streamers map[string]StreamerInfo, duration time.Duration) *OnlineListResults {
	return &OnlineListResults{Streamers: streamers, Duration: duration}
}

// NewOnlineListResultsFailed creates a failed OnlineListResults
func NewOnlineListResultsFailed() *OnlineListResults {
	return &OnlineListResults{Failed: true}
}
