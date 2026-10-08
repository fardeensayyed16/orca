package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

type Server struct {
	store storage.Storage
	mux   *http.ServeMux
	token string
}

func NewServer(store storage.Storage, token string) *Server {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
		token: token,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// No-auth routes
	s.mux.HandleFunc("GET /api/v1/ping", s.handlePing)
	s.mux.HandleFunc("GET /api/v1/_test/list", s.handleTestList)
	s.mux.HandleFunc("GET /api/v1/_test/echo", s.handleTestEcho)

	// Token-protected routes (Phase 3)
	s.mux.HandleFunc("GET /api/v1/nodes", s.auth(s.handleListNodes))

	// token/show — only callable on loopback --server; token auth still required
	s.mux.HandleFunc("POST /api/v1/token/show", s.auth(s.handleTokenShow))
}

// auth wraps a handler with bearer-token enforcement using constant-time compare.
func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	want := []byte(s.token)
	return func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(hdr) <= len(prefix) || hdr[:len(prefix)] != prefix {
			jsonError(w, "missing or malformed Authorization header", http.StatusUnauthorized)
			return
		}
		got := []byte(hdr[len(prefix):])
		if subtle.ConstantTimeCompare(got, want) != 1 {
			jsonError(w, "invalid token", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

// ServeHTTP makes Server implement http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ---- handlers ----

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleTestList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "smoke-test-ok"})
}

func (s *Server) handleTestEcho(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"echo": r.URL.Query().Get("msg")})
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes(r.Context())
	if err != nil {
		jsonError(w, "internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Never return null — return an empty array
	if nodes == nil {
		nodes = []*types.Node{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

func (s *Server) handleTokenShow(w http.ResponseWriter, r *http.Request) {
	tok, err := s.store.GetToken(r.Context())
	if err != nil {
		jsonError(w, "internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": tok})
}

// jsonError writes a JSON {"error":"..."} response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
