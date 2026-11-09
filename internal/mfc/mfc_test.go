package mfc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	types "mfcbot/internal"
	"mfcbot/internal/config"
)

func buildFrame(fcType int, payload string) []byte {
	body := fmt.Sprintf("%d 0 0 0 0 %s", fcType, url.QueryEscape(payload))
	return []byte(fmt.Sprintf("%06d%s", len(body), body))
}

func TestWalkFrames(t *testing.T) {
	first := buildFrame(mfcFCTypeSessionState, `{"uid":123,"nm":"Alice","vs":0}`)
	second := buildFrame(mfcFCTypeRoomData, `{"123":42}`)
	data := append(append([]byte{}, first...), second...)

	var typesSeen []int
	consumed, err := walkFrames(data, func(f frame) bool {
		typesSeen = append(typesSeen, f.fcType)
		return false
	})
	if err != nil {
		t.Fatalf("walkFrames error: %v", err)
	}
	if consumed != len(data) {
		t.Fatalf("consumed = %d, want %d", consumed, len(data))
	}
	if len(typesSeen) != 2 || typesSeen[0] != mfcFCTypeSessionState || typesSeen[1] != mfcFCTypeRoomData {
		t.Fatalf("types seen = %v", typesSeen)
	}

	// A truncated trailing frame must be reported as a clean partial.
	partial := append(append([]byte{}, first...), second[:len(second)-3]...)
	consumed, err = walkFrames(partial, func(frame) bool { return false })
	if err != nil {
		t.Fatalf("partial walkFrames error: %v", err)
	}
	if consumed != len(first) {
		t.Fatalf("partial consumed = %d, want %d", consumed, len(first))
	}
}

func TestParseSessionState(t *testing.T) {
	f, ok := parseFrame([]byte(`20 0 0 0 0 ` + url.QueryEscape(`{"uid":7,"nm":"Bob","vs":13,"m":{"rc":9,"topic":"Hi%20there"}}`)))
	if !ok {
		t.Fatal("parseFrame failed")
	}
	msg, err := decodeFrame(f)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	update := msg.(*sessionStateMsg).update
	if update.UID == nil || *update.UID != 7 {
		t.Fatalf("uid = %v", update.UID)
	}
	if update.Name == nil || *update.Name != "Bob" {
		t.Fatalf("name = %v", update.Name)
	}
	if rc := update.rc(); rc == nil || *rc != 9 {
		t.Fatalf("rc = %v", rc)
	}
	if kind := mfcShowKind(*update.VS); kind != types.ShowGroup {
		t.Fatalf("show kind = %v", kind)
	}
	if decoded := decodeMFCTopic(*update.topic()); decoded != "Hi there" {
		t.Fatalf("topic = %q", decoded)
	}
}

func TestAdapterHTTPEndpoints(t *testing.T) {
	cfg := &config.Config{MaxSnapshotSize: 100, ExtDataFetchTimeout: 1}
	adapter := NewAdapter(cfg)
	adapter.state.online[42] = &streamer{name: "Alice", uid: 42, vs: 0, rc: 5, topic: "Hello"}
	adapter.state.bulkApplied = true

	srv := httptest.NewServer(adapter.buildMux())
	defer srv.Close()

	get := func(path string) (int, string) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	if code, body := get("/healthz"); code != http.StatusOK || body != "ok\n" {
		t.Fatalf("/healthz = %d %q", code, body)
	}
	if code, _ := get("/version"); code != http.StatusOK {
		t.Fatalf("/version status = %d", code)
	}

	code, body := get("/online")
	if code != http.StatusOK {
		t.Fatalf("/online status = %d", code)
	}
	var online types.OnlineListResults
	if err := json.Unmarshal([]byte(body), &online); err != nil {
		t.Fatalf("decode /online: %v\n%s", err, body)
	}
	if online.Failed {
		t.Fatal("/online unexpectedly failed")
	}
	info, ok := online.Streamers["alice"]
	if !ok {
		t.Fatalf("missing alice in %v", online.Streamers)
	}
	if info.UID != 42 || info.Subject != "Hello" || info.ShowKind != types.ShowPublic {
		t.Fatalf("unexpected streamer: %+v", info)
	}

	// No live websocket session: /status must degrade to 503, not 200/500.
	if code, _ := get("/status?name=Alice"); code != http.StatusServiceUnavailable {
		t.Fatalf("/status status = %d, want 503", code)
	}
	if code, _ := get("/status"); code != http.StatusBadRequest {
		t.Fatalf("/status missing name = %d, want 400", code)
	}
	if code, _ := get("/status?name=bad%20name"); code != http.StatusBadRequest {
		t.Fatalf("/status bad name = %d, want 400", code)
	}
}

func TestGetOnlineEmptyBeforeBulk(t *testing.T) {
	adapter := NewAdapter(&config.Config{MaxSnapshotSize: 10})
	if got := adapter.GetOnline(); len(got) != 0 {
		t.Fatalf("expected empty before bulk, got %v", got)
	}
	if _, err := adapter.LookupModelID(context.Background(), "Alice"); err != errNoSession {
		t.Fatalf("LookupModelID error = %v, want errNoSession", err)
	}
}

func TestStartHeadlessWhenListenAddressEmpty(t *testing.T) {
	adapter := NewAdapter(&config.Config{MaxSnapshotSize: 10})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // workers exit immediately without touching the network
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start headless: %v", err)
	}
	adapter.Stop()
}

func TestStartInvalidListenAddress(t *testing.T) {
	adapter := NewAdapter(&config.Config{
		MaxSnapshotSize: 10,
		ListenAddress:   "not a valid address",
	})
	if err := adapter.Start(context.Background()); err == nil {
		t.Fatal("Start with an invalid address should fail")
	}
	adapter.Stop()
}
