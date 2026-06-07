package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/zabbix"
)

type Server struct {
	store *collector.Store
}

func New(store *collector.Store) *Server {
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/zabbix/discovery", s.discovery)
	mux.HandleFunc("/zabbix/job/", s.job)
	mux.HandleFunc("/api/status", s.status)
	mux.HandleFunc("/api/jobs", s.jobs)
	return mux
}

func (s *Server) snapshot(w http.ResponseWriter) (collector.Snapshot, bool) {
	snapshot, ok := s.store.Get()
	if !ok {
		http.Error(w, "no snapshot available", http.StatusServiceUnavailable)
		return collector.Snapshot{}, false
	}
	return snapshot, true
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, snapshot.Health)
}

func (s *Server) discovery(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w)
	if !ok {
		return
	}
	data, err := zabbix.DiscoveryJSON(snapshot, r.URL.Query().Get("product"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func (s *Server) job(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/zabbix/job/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "expected /zabbix/job/{product}/{task_id}", http.StatusBadRequest)
		return
	}
	job, found := zabbix.FindJob(snapshot, parts[0], parts[1])
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, job)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, snapshot)
}

func (s *Server) jobs(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, snapshot.Jobs)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}
