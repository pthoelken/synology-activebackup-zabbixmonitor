package apiserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/zabbix"
)

type Server struct {
	mu         sync.RWMutex
	cfg        config.Config
	configPath string
	store      *collector.Store
	logger     *slog.Logger
}

func New(cfg config.Config, configPath string, store *collector.Store, logger *slog.Logger) *Server {
	return &Server{
		cfg:        cfg,
		configPath: configPath,
		store:      store,
		logger:     logger,
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	cfg := s.Config()
	addr := net.JoinHostPort(cfg.API.Bind, strconv.Itoa(cfg.API.Port))
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		s.logger.Info("api server started", "addr", addr)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errc <- err
			return
		}
		errc <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errc:
		return err
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ping", s.withAuth(s.handlePing))
	mux.HandleFunc("/api/v1/status", s.withAuth(s.handleStatus))
	mux.HandleFunc("/api/v1/discovery", s.withAuth(s.handleDiscovery))
	mux.HandleFunc("/api/v1/health", s.withAuth(s.handleHealth))
	mux.HandleFunc("/api/v1/summary", s.withAuth(s.handleSummary))
	mux.HandleFunc("/api/v1/job", s.withAuth(s.handleJob))
	mux.HandleFunc("/api/v1/sender-log", s.withAuth(s.handleSenderLog))
	mux.HandleFunc("/api/v1/config", s.withAuth(s.handleConfig))
	return mux
}

func (s *Server) Config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Server) setConfig(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "missing or invalid API token")
			return
		}
		next(w, r)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	candidate := ""
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		candidate = strings.TrimSpace(auth[7:])
	}
	if candidate == "" {
		candidate = strings.TrimSpace(r.Header.Get("X-API-Token"))
	}
	if candidate == "" {
		return false
	}
	if s.tokenMatches(candidate, s.Config().API.Token) {
		return true
	}

	// Package upgrades and DSM wizard updates can write config before a running
	// service has reloaded it. Refresh once on auth failure to avoid stale-token
	// 401s in the DSM app.
	if s.configPath != "" {
		cfg, err := config.Load(s.configPath)
		if err == nil {
			s.setConfig(cfg)
			return s.tokenMatches(candidate, cfg.API.Token)
		}
	}
	return false
}

func (s *Server) tokenMatches(candidate string, token string) bool {
	if token == "" || len(candidate) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}

func (s *Server) snapshot() (collector.Snapshot, bool) {
	if s.store == nil {
		return collector.Snapshot{}, false
	}
	return s.store.Get()
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot, ok := s.snapshot()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no snapshot available")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot, ok := s.snapshot()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no snapshot available")
		return
	}
	data, err := zabbix.DiscoveryJSON(snapshot, r.URL.Query().Get("product"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeRaw(w, http.StatusOK, "application/json; charset=utf-8", data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot, ok := s.snapshot()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no snapshot available")
		return
	}
	field := r.URL.Query().Get("field")
	if field == "" {
		field = "json"
	}
	value, err := zabbix.HealthField(snapshot, field, r.URL.Query().Get("product"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if field == "json" {
		writeRaw(w, http.StatusOK, "application/json; charset=utf-8", []byte(value))
		return
	}
	writeText(w, http.StatusOK, value)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot, ok := s.snapshot()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no snapshot available")
		return
	}
	field := r.URL.Query().Get("field")
	if field == "" {
		field = "total"
	}
	value, err := zabbix.SummaryField(snapshot, field)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeText(w, http.StatusOK, value)
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot, ok := s.snapshot()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no snapshot available")
		return
	}
	product := r.URL.Query().Get("product")
	taskID := r.URL.Query().Get("task_id")
	field := r.URL.Query().Get("field")
	if product == "" || taskID == "" {
		writeError(w, http.StatusBadRequest, "product and task_id are required")
		return
	}
	job, found := zabbix.FindJob(snapshot, product, taskID)
	if !found {
		if field == "status" || field == "" {
			writeText(w, http.StatusOK, strconv.Itoa(collector.StatusNoData))
			return
		}
		writeText(w, http.StatusOK, "0")
		return
	}
	if field == "" || field == "json" || field == "info" {
		if field == "info" {
			value, err := zabbix.JobField(job, field)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeRaw(w, http.StatusOK, "application/json; charset=utf-8", []byte(value))
			return
		}
		writeJSON(w, http.StatusOK, job)
		return
	}
	value, err := zabbix.JobField(job, field)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeText(w, http.StatusOK, value)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.Config())
	case http.MethodPut, http.MethodPost:
		var next config.Config
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		current := s.Config()
		if next.API.Token == "" {
			token, err := config.GenerateAPIToken()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			next.API.Token = token
		}
		if err := config.Write(s.configPath, next); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		reloaded, err := config.Load(s.configPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.setConfig(reloaded)
		restartRequired := !configsEqual(current, reloaded)
		writeJSON(w, http.StatusOK, map[string]any{
			"config":           reloaded,
			"restart_required": restartRequired,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSenderLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	log, err := zabbix.ReadSenderLog(s.Config().Paths.SenderLogFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func configsEqual(a config.Config, b config.Config) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeRaw(w, status, "application/json; charset=utf-8", data)
}

func writeText(w http.ResponseWriter, status int, value string) {
	writeRaw(w, status, "text/plain; charset=utf-8", []byte(value+"\n"))
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeRaw(w, status, "application/json; charset=utf-8", []byte(fmt.Sprintf(`{"error":%q}`, message)))
}

func writeRaw(w http.ResponseWriter, status int, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
