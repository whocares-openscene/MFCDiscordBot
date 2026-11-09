package mfc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	types "mfcbot/internal"
	"mfcbot/internal/config"
	"mfcbot/internal/logging"
)

// External resource URLs and connection-setup constants.
const (
	mfcServerConfigURL = "https://www.myfreecams.com/_js/serverconfig.js"
	mfcExtDataURL      = "https://www.myfreecams.com/php/FcwExtResp.php?respkey=%d&type=%d&opts=%d&serv=%d"

	mfcWSOrigin = "https://m.myfreecams.com"

	wsReadLimit             = 8 * 1024 * 1024
	wsKeepAliveEvery        = 15 * time.Second
	reconnectBackoffInitial = time.Second
	httpReadTimeout         = 10 * time.Second
	httpWriteTimeout        = 30 * time.Second
	httpShutdownTimeout     = 5 * time.Second
	wsWriteTimeout          = 10 * time.Second
	pendingLookupTTL        = time.Hour
	nameCachePruneEvery     = time.Hour
	videoHostsRefreshEvery  = 10 * time.Minute
	statusLookupTimeout     = 5 * time.Second
)

// mfcUsernameRegexp restricts /status's name parameter.
var mfcUsernameRegexp = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// wsSession owns a single MFC websocket connection plus per-connection state.
type wsSession struct {
	conn        *websocket.Conn
	dialedAt    time.Time
	writeMu     sync.Mutex
	nextQueryID atomic.Int64
	pendingMu   sync.Mutex
	pending     map[int64]chan *lookupResponseMsg
}

func newWSSession(conn *websocket.Conn) *wsSession {
	return &wsSession{
		conn:     conn,
		dialedAt: time.Now(),
		pending:  map[int64]chan *lookupResponseMsg{},
	}
}

func (s *wsSession) registerPendingLookup(qid int64) chan *lookupResponseMsg {
	ch := make(chan *lookupResponseMsg, 1)
	s.pendingMu.Lock()
	s.pending[qid] = ch
	s.pendingMu.Unlock()
	return ch
}

func (s *wsSession) cancelPendingLookup(qid int64) {
	s.pendingMu.Lock()
	delete(s.pending, qid)
	s.pendingMu.Unlock()
}

func (s *wsSession) deliverLookup(qid int, msg *lookupResponseMsg) bool {
	s.pendingMu.Lock()
	ch, ok := s.pending[int64(qid)]
	if ok {
		delete(s.pending, int64(qid))
	}
	s.pendingMu.Unlock()
	if !ok {
		return false
	}
	ch <- msg
	return true
}

func (s *wsSession) closePendingLookups() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for qid, ch := range s.pending {
		close(ch)
		delete(s.pending, qid)
	}
}

func (s *wsSession) write(ctx context.Context, payload string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.Write(ctx, websocket.MessageText, []byte(payload))
}

func closeAndLog(conn *websocket.Conn, code websocket.StatusCode, reason, op string) {
	if err := conn.Close(code, reason); err != nil {
		logging.Ltrace("%s: %v", op, err)
	}
}

func (s *wsSession) requestNameLookup(ctx context.Context, uid int) error {
	qid := s.nextQueryID.Add(1)
	frame := encodeUsernameLookupByUID(qid, uid)
	writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	if err := s.write(writeCtx, frame); err != nil {
		return fmt.Errorf("lookup write failed for uid %d, %w", uid, err)
	}
	return nil
}

func (s *wsSession) lookupByName(ctx context.Context, name string) (*lookupResponseMsg, error) {
	qid := s.nextQueryID.Add(1)
	ch := s.registerPendingLookup(qid)
	frame := encodeUsernameLookupByName(qid, name)
	writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	err := s.write(writeCtx, frame)
	cancel()
	if err != nil {
		s.cancelPendingLookup(qid)
		return nil, fmt.Errorf("lookup write failed for name %s, %w", name, err)
	}
	select {
	case <-ctx.Done():
		s.cancelPendingLookup(qid)
		return nil, ctx.Err()
	case msg, ok := <-ch:
		if !ok {
			return nil, errors.New("session ended before lookup reply")
		}
		return msg, nil
	}
}

// fetchBounded issues req and reads up to limit bytes.
func fetchBounded(req *http.Request, client *http.Client, limit int) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read response, %w", err)
	}
	if len(body) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return body, nil
}

type mfcServerConfig struct {
	wsServer   string
	videoHosts map[int]string
}

func fetchMFCServerConfig(
	ctx context.Context,
	state *onlineState,
	client *http.Client,
	cfg *config.Config,
) (_ *mfcServerConfig, err error) {
	defer func() {
		if err != nil {
			state.lifetimeServerConfigFailures.Add(1)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, "GET", mfcServerConfigURL, nil)
	if err != nil {
		return nil, err
	}
	body, err := fetchBounded(req, client, 8*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("fetch serverconfig: %w", err)
	}
	var raw struct {
		WebSocketServers map[string]string `json:"websocket_servers"`
		H5VideoServers   map[string]string `json:"h5video_servers"`
		NgVideoServers   map[string]string `json:"ngvideo_servers"`
		WzExtServers     map[string]string `json:"wzext_servers"`
		WzObsServers     map[string]string `json:"wzobs_servers"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse serverconfig, %w", err)
	}
	wsServers := make([]string, 0, len(raw.WebSocketServers))
	for name, proto := range raw.WebSocketServers {
		if proto == "rfc6455" {
			wsServers = append(wsServers, name)
		}
	}
	if len(wsServers) == 0 {
		return nil, errors.New("no websocket servers in serverconfig")
	}
	videoHosts := make(map[int]string)
	for _, m := range []map[string]string{
		raw.H5VideoServers, raw.NgVideoServers,
		raw.WzExtServers, raw.WzObsServers,
	} {
		for csStr, host := range m {
			cs, err := strconv.Atoi(csStr)
			if err != nil {
				continue
			}
			videoHosts[cs] = host
		}
	}
	return &mfcServerConfig{
		wsServer:   wsServers[rand.IntN(len(wsServers))],
		videoHosts: videoHosts,
	}, nil
}

func fetchExtData(
	ctx context.Context,
	client *http.Client,
	cfg *config.Config,
	ext *mfcExtData,
) (bulk, error) {
	u := fmt.Sprintf(mfcExtDataURL, ext.RespKey, ext.Type, ext.Opts, ext.Serv)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	body, err := fetchBounded(req, client, 8*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("fetch extdata: %w", err)
	}
	var list struct {
		RData []any `json:"rdata"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse extdata, %w", err)
	}
	return parseBulkRData(list.RData)
}

// dialMFC picks a random MFC websocket server, dials it, and completes the
// FCS handshake and guest login.
func dialMFC(ctx context.Context, state *onlineState, client *http.Client, cfg *config.Config) (*websocket.Conn, map[int]string, error) {
	sc, err := fetchMFCServerConfig(ctx, state, client, cfg)
	if err != nil {
		return nil, nil, err
	}
	wsURL := fmt.Sprintf("wss://%s.myfreecams.com/fcsl", sc.wsServer)
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: client,
		HTTPHeader: http.Header{"Origin": []string{mfcWSOrigin}},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("ws dial %s, %w", wsURL, err)
	}
	conn.SetReadLimit(wsReadLimit)

	if err := conn.Write(ctx, websocket.MessageText, []byte(mfcWSHandshakeFrame)); err != nil {
		closeAndLog(conn, websocket.StatusInternalError, "", "handshake close")
		return nil, nil, fmt.Errorf("ws handshake, %w", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(encodeLogin())); err != nil {
		closeAndLog(conn, websocket.StatusInternalError, "", "login close")
		return nil, nil, fmt.Errorf("ws login, %w", err)
	}
	logging.Linf("connected to %s", wsURL)
	return conn, sc.videoHosts, nil
}

// translateWSError converts a coder/websocket read error into a description.
func translateWSError(err error, idleTimeout time.Duration) error {
	if status := websocket.CloseStatus(err); status != -1 {
		return fmt.Errorf("server closed connection (code=%d)", status)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("idle timeout: no frames in %s", idleTimeout)
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("shutdown in progress")
	}
	return err
}

// runFrameLoop reads frames from conn with an idle timeout.
func runFrameLoop(
	ctx context.Context,
	conn *websocket.Conn,
	idleTimeout time.Duration,
	handle func(data []byte) (stop bool, err error),
) error {
	for {
		readCtx, readCancel := context.WithTimeout(ctx, idleTimeout)
		_, data, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			return translateWSError(err, idleTimeout)
		}
		stop, err := handle(data)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// runKeepAlive writes a NULL frame on a ticker.
func runKeepAlive(ctx context.Context, sess *wsSession) {
	t := time.NewTicker(wsKeepAliveEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
			err := sess.write(writeCtx, mfcNullFrame)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logging.Linf("keepalive write failed: %v, closing for reconnect", err)
				closeAndLog(sess.conn, websocket.StatusInternalError, "keepalive write failed", "keepalive close")
				return
			}
		}
	}
}

// runBulkWatchdog forces a reconnect when the initial bulk dump has not landed.
func runBulkWatchdog(ctx context.Context, state *onlineState, sess *wsSession, timeout time.Duration) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return
	case <-t.C:
		if state.bulkReady() {
			return
		}
		logging.Lerr("no bulk dump within %s of connecting, closing for reconnect", timeout)
		closeAndLog(sess.conn, websocket.StatusInternalError, "bulk arrival timeout", "bulk watchdog close")
	}
}

// runWebsocketSession dials, runs keepalive, and feeds each parsed frame to
// handle.
func runWebsocketSession(
	ctx context.Context,
	state *onlineState,
	cfg *config.Config,
	client *http.Client,
	handle func(sessCtx context.Context, sess *wsSession, f frame) (stop bool, err error),
) (err error) {
	dialCtx, dialCancel := context.WithTimeout(ctx, cfg.WSConnectTimeout)
	conn, videoHosts, err := dialMFC(dialCtx, state, client, cfg)
	dialCancel()
	if err != nil {
		return err
	}
	state.setVideoHosts(videoHosts)
	sess := newWSSession(conn)
	state.currentSession.Store(sess)

	connCtx, connCancel := context.WithCancel(ctx)
	keepAliveDone := make(chan struct{})
	go func() {
		defer close(keepAliveDone)
		runKeepAlive(connCtx, sess)
	}()
	go runBulkWatchdog(connCtx, state, sess, cfg.BulkArrivalTimeout)
	defer func() {
		state.currentSession.CompareAndSwap(sess, nil)
		sess.closePendingLookups()
		connCancel()
		<-keepAliveDone
		status := websocket.StatusNormalClosure
		if err != nil && ctx.Err() == nil {
			status = websocket.StatusInternalError
		}
		closeAndLog(conn, status, "", "session close")
	}()

	var buf bytes.Buffer
	return runFrameLoop(connCtx, conn, cfg.WSIdleTimeout, func(data []byte) (bool, error) {
		buf.Write(data)
		var (
			stopErr error
			stopOK  bool
		)
		consumed, walkErr := walkFrames(buf.Bytes(), func(f frame) bool {
			stop, err := handle(connCtx, sess, f)
			if err != nil {
				stopErr = err
				return true
			}
			if stop {
				stopOK = true
				return true
			}
			return false
		})
		if stopErr != nil {
			return false, stopErr
		}
		if walkErr != nil {
			return false, walkErr
		}
		buf.Next(consumed)
		if stopOK {
			return true, nil
		}
		return false, state.validate()
	})
}

// dispatchFrame routes one parsed FCS frame to its handler.
func dispatchFrame(
	sessCtx context.Context,
	state *onlineState,
	cfg *config.Config,
	client *http.Client,
	sess *wsSession,
	f frame,
) error {
	if f.fcType == mfcFCTypeNull {
		return nil
	}
	msg, err := decodeFrame(f)
	if err != nil {
		return fmt.Errorf("decode @fctype = %d, %w", f.fcType, err)
	}
	if msg == nil {
		return nil
	}
	switch m := msg.(type) {
	case *bulkRefMsg:
		toLookup, err := state.applyBulk(sessCtx, cfg, client, m.ext)
		if err != nil {
			return err
		}
		for _, uid := range toLookup {
			if err := sess.requestNameLookup(sessCtx, uid); err != nil {
				return err
			}
		}
		return nil
	case *sessionStateMsg:
		if err := state.applyUpdate(m.update, cfg); err != nil {
			return err
		}
		if m.update.UID != nil && state.claimLookup(*m.update.UID, cfg.LookupTTL) {
			if err := sess.requestNameLookup(sessCtx, *m.update.UID); err != nil {
				return err
			}
		}
		return nil
	case *lookupResponseMsg:
		if sess.deliverLookup(m.qid, m) {
			return nil
		}
		return state.applyUpdate(m.update, cfg)
	case *roomDataMsg:
		state.applyRoomData(m.counts)
		return nil
	}
	return nil
}

// manageWebsocketSessions keeps a websocket connected for the lifetime of ctx.
func manageWebsocketSessions(ctx context.Context, state *onlineState, cfg *config.Config, client *http.Client) {
	backoff := reconnectBackoffInitial
	for ctx.Err() == nil {
		err := runWebsocketSession(ctx, state, cfg, client, func(sessCtx context.Context, sess *wsSession, f frame) (bool, error) {
			return false, dispatchFrame(sessCtx, state, cfg, client, sess, f)
		})
		if state.markDisconnected() {
			backoff = reconnectBackoffInitial
		}
		if ctx.Err() != nil {
			return
		}
		logging.Linf("%v; reconnecting in %s", err, backoff)
		state.lifetimeDisconnects.Add(1)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > cfg.ReconnectBackoffMax {
			backoff = cfg.ReconnectBackoffMax
		}
	}
}

// runNameCachePruner periodically drops expired entries from the name cache.
func runNameCachePruner(ctx context.Context, state *onlineState, cfg *config.Config) {
	t := time.NewTicker(nameCachePruneEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			state.pruneNameCache(cfg.NameCacheTTL, pendingLookupTTL)
		}
	}
}

// runVideoHostsRefresher periodically re-fetches MFC's serverconfig.js.
func runVideoHostsRefresher(ctx context.Context, state *onlineState, cfg *config.Config, client *http.Client) {
	t := time.NewTicker(videoHostsRefreshEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sc, err := fetchMFCServerConfig(ctx, state, client, cfg)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logging.Linf("refresh video hosts: %v", err)
				continue
			}
			state.setVideoHosts(sc.videoHosts)
		}
	}
}

// LookupStatus performs an MFC username status lookup for the /status route.
func (a *Adapter) lookupStatus(ctx context.Context, name string) (types.StreamerInfoWithStatus, error) {
	sess := a.state.currentSession.Load()
	if sess == nil {
		return types.StreamerInfoWithStatus{Status: types.StatusUnknown}, errNoSession
	}
	lookupCtx, cancel := context.WithTimeout(ctx, statusLookupTimeout)
	defer cancel()
	reply, err := sess.lookupByName(lookupCtx, name)
	if err != nil {
		return types.StreamerInfoWithStatus{Status: types.StatusUnknown}, err
	}
	return types.StreamerInfoWithStatus{Status: mfcLookupStatus(reply, name)}, nil
}

// mfcLookupStatus maps a USERNAMELOOKUP reply to a StatusKind.
func mfcLookupStatus(m *lookupResponseMsg, queriedName string) types.StatusKind {
	if string(m.payload) == queriedName {
		return types.StatusNotFound
	}
	if m.update.UID == nil || *m.update.UID == 0 {
		return types.StatusNotFound
	}
	if m.update.VS != nil && *m.update.VS == mfcFCVideoOffline {
		return types.StatusOffline
	}
	return types.StatusOnline
}
