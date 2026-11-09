package mfc

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	types "mfcbot/internal"
	"mfcbot/internal/config"
)

var (
	errNoSession = errors.New("no live mfc session")
	errNotFound  = errors.New("model not found")
	errBadName   = errors.New("invalid model name")
)

// Adapter owns the MFC websocket connection, the online snapshot state, and
// the HTTP endpoints (/online, /status, /healthz, /version).
type Adapter struct {
	cfg    *config.Config
	state  *onlineState
	client *http.Client
	srv    *http.Server
	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewAdapter creates a new MFC adapter.
func NewAdapter(cfg *config.Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		state:  newOnlineState(),
		client: &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
	}
}

// GetOnline returns the current online streamers map. It returns an empty map
// until the first bulk dump has been applied.
func (a *Adapter) GetOnline() map[string]types.StreamerInfo {
	streamers, ok := a.state.collectIfReady()
	if !ok {
		return map[string]types.StreamerInfo{}
	}
	return streamers
}

// GetStatus performs a live existence/status lookup for a model name.
func (a *Adapter) GetStatus(ctx context.Context, name string) (types.StreamerInfoWithStatus, error) {
	return a.lookupStatus(ctx, name)
}

// OnlineReady reports whether the current websocket session has applied its
// initial bulk snapshot, i.e. whether GetOnline can be trusted.
func (a *Adapter) OnlineReady() bool {
	return a.state.bulkReady()
}

// LookupModelID resolves a model username to its numeric MFC id using the
// live websocket session.
func (a *Adapter) LookupModelID(ctx context.Context, name string) (int, error) {
	if !mfcUsernameRegexp.MatchString(name) {
		return 0, errBadName
	}
	sess := a.state.currentSession.Load()
	if sess == nil {
		return 0, errNoSession
	}
	lookupCtx, cancel := context.WithTimeout(ctx, statusLookupTimeout)
	defer cancel()
	reply, err := sess.lookupByName(lookupCtx, name)
	if err != nil {
		return 0, err
	}
	if reply.update.UID == nil || *reply.update.UID == 0 {
		return 0, errNotFound
	}
	return *reply.update.UID, nil
}

// Start launches the HTTP server (when LISTEN_ADDRESS is set) and the
// background websocket workers. A listener is optional: the Discord bot and
// MFC monitor work without it, so an empty ListenAddress runs headless.
func (a *Adapter) Start(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	a.cancel = cancel

	// Bind synchronously so a bad address or busy port surfaces from Start
	// instead of leaving the bot running without its HTTP API.
	var ln net.Listener
	if a.cfg.ListenAddress != "" {
		a.srv = &http.Server{
			Addr:         a.cfg.ListenAddress,
			Handler:      a.buildMux(),
			ReadTimeout:  httpReadTimeout,
			WriteTimeout: httpWriteTimeout,
		}
		var err error
		ln, err = net.Listen("tcp", a.cfg.ListenAddress)
		if err != nil {
			cancel()
			return err
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manageWebsocketSessions(ctx, a.state, a.cfg, a.client)
	}()
	wg.Add(1)
	go func() { defer wg.Done(); runNameCachePruner(ctx, a.state, a.cfg) }()
	wg.Add(1)
	go func() { defer wg.Done(); runVideoHostsRefresher(ctx, a.state, a.cfg, a.client) }()

	if ln == nil {
		log.Printf("HTTP server disabled (LISTEN_ADDRESS is not set)")
		return nil
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		defer shutdownCancel()
		_ = a.srv.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Printf("Starting HTTP server on %s", a.cfg.ListenAddress)
		if err := a.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
		wg.Wait()
	}()

	return nil
}

// Stop cancels the background workers and shuts down the HTTP server.
func (a *Adapter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// buildMux wires up the HTTP routes.
func (a *Adapter) buildMux() http.Handler {
	mux := http.NewServeMux()

	// /online endpoint — current snapshot as JSON
	mux.HandleFunc("/online", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if streamers, ok := a.state.collectIfReady(); ok {
			_ = json.NewEncoder(w).Encode(types.NewOnlineListResults(streamers, 0))
		} else {
			_ = json.NewEncoder(w).Encode(types.NewOnlineListResultsFailed())
		}
	})

	// /status endpoint — backs the bot's QueryStatus for MFC
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "missing name", http.StatusBadRequest)
			return
		}
		if !mfcUsernameRegexp.MatchString(name) {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		result, err := a.lookupStatus(r.Context(), name)
		if err != nil {
			switch {
			case errors.Is(err, errNoSession):
				http.Error(w, "no live mfc session", http.StatusServiceUnavailable)
			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, "lookup timeout", http.StatusGatewayTimeout)
			default:
				http.Error(w, "lookup failed", http.StatusBadGateway)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	// /healthz endpoint — liveness probe
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	// /version endpoint
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("mfcbot v1.0.0\n"))
	})

	return mux
}
