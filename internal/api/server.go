package api

import (
	"encoding/json"
	"net/http"

	"github.com/fardeensayyed16/orca/internal/storage"
)

type Server struct {
	store storage.Storage
	mux   *http.ServeMux
}

func NewServer(store storage.Storage) *Server {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Ping endpoint (no auth)
	s.mux.HandleFunc("GET /api/v1/ping", s.handlePing)

	// Throwaway smoke-test endpoints /_test/* (no auth)
	s.mux.HandleFunc("GET /api/v1/_test/list", s.handleTestList)
	s.mux.HandleFunc("GET /api/v1/_test/echo", s.handleTestEcho)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleTestList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "smoke-test-ok"})
}

func (s *Server) handleTestEcho(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"echo": r.URL.Query().Get("msg")})
}

