package notifier

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	types "mfcbot/internal"
	"mfcbot/internal/database"
)

type fakeSender struct {
	messages []string
	files    []string
}

func (f *fakeSender) SendMessage(channelID, content string) error {
	f.messages = append(f.messages, content)
	return nil
}

func (f *fakeSender) SendMessageWithFile(channelID, content, filename string, data []byte) error {
	f.messages = append(f.messages, content)
	f.files = append(f.files, filename)
	return nil
}

type fixture struct {
	db     *database.Database
	sender *fakeSender
	n      *Notifier
	online map[int]types.StreamerInfo
	ready  bool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	f := &fixture{
		db:     db,
		sender: &fakeSender{},
		online: map[int]types.StreamerInfo{},
		ready:  true,
	}
	f.n = New(db, f.sender, func() map[string]types.StreamerInfo {
		out := make(map[string]types.StreamerInfo, len(f.online))
		for _, v := range f.online {
			out[strings.ToLower(v.Subject)] = v
		}
		return out
	}, func() bool { return f.ready }, time.Minute)
	return f
}

func (f *fixture) check() {
	f.n.check(context.Background())
}

func TestPublicShowNotification(t *testing.T) {
	f := newFixture(t)
	if _, err := f.db.AddModel("1", "Alice", "Come watch", "chan", 0, false); err != nil {
		t.Fatal(err)
	}
	f.online[1] = types.StreamerInfo{UID: 1, ShowKind: types.ShowPublic, Subject: "Hello there"}

	f.check()

	if len(f.sender.messages) != 1 {
		t.Fatalf("messages = %v", f.sender.messages)
	}
	want := "Come watch\nCurrent topic is:\nHello there"
	if f.sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", f.sender.messages[0], want)
	}
	m, err := f.db.GetModel("1", "chan")
	if err != nil {
		t.Fatal(err)
	}
	if m.Topic != "Public" {
		t.Fatalf("topic = %q, want Public", m.Topic)
	}
	if m.Time == 0 {
		t.Fatal("time was not set")
	}

	// A second check with no state change must not re-announce.
	f.check()
	if len(f.sender.messages) != 1 {
		t.Fatalf("duplicate notification: %v", f.sender.messages)
	}
}

func TestPublicShowWithoutTopicIsSkipped(t *testing.T) {
	f := newFixture(t)
	f.db.AddModel("1", "Alice", "msg", "chan", 0, false)
	f.online[1] = types.StreamerInfo{UID: 1, ShowKind: types.ShowPublic, Subject: ""}

	f.check()
	if len(f.sender.messages) != 0 {
		t.Fatalf("unexpected messages: %v", f.sender.messages)
	}
}

func TestGroupAndOfflineNotifications(t *testing.T) {
	f := newFixture(t)
	f.db.AddModel("1", "Alice", "msg", "chan", 0, false)

	f.online[1] = types.StreamerInfo{UID: 1, ShowKind: types.ShowGroup}
	f.check()
	if len(f.sender.messages) != 1 || !strings.Contains(f.sender.messages[0], "group show") {
		t.Fatalf("messages = %v", f.sender.messages)
	}
	if m, _ := f.db.GetModel("1", "chan"); m.Topic != "2" {
		t.Fatalf("topic = %q, want 2", m.Topic)
	}

	delete(f.online, 1)
	f.check()
	if len(f.sender.messages) != 2 || !strings.Contains(f.sender.messages[1], "has gone offline") {
		t.Fatalf("messages = %v", f.sender.messages)
	}
	if m, _ := f.db.GetModel("1", "chan"); m.Topic != "" || m.Time != 0 {
		t.Fatalf("model not reset: %+v", m)
	}
}

func TestOfflineWhileBotWasGone(t *testing.T) {
	f := newFixture(t)
	f.db.AddModel("1", "Alice", "msg", "chan", 0, false)
	f.db.UpdateTime("1", time.Now().Add(-time.Hour).UnixMilli(), "chan")

	f.check()
	if len(f.sender.messages) != 1 || !strings.Contains(f.sender.messages[0], "while the bot was offline") {
		t.Fatalf("messages = %v", f.sender.messages)
	}
	if m, _ := f.db.GetModel("1", "chan"); m.Time != 0 {
		t.Fatalf("time not reset: %d", m.Time)
	}
}

func TestCheckWaitsForSnapshot(t *testing.T) {
	f := newFixture(t)
	f.ready = false
	f.db.AddModel("1", "Alice", "msg", "chan", 0, false)
	f.db.UpdateTime("1", time.Now().Add(-time.Hour).UnixMilli(), "chan")

	f.check()
	if len(f.sender.messages) != 0 {
		t.Fatalf("checked before snapshot was ready: %v", f.sender.messages)
	}
}
