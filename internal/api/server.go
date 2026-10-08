package api

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"time"

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

	// Token-protected node routes
	s.mux.HandleFunc("POST /api/v1/nodes/register", s.auth(s.handleRegisterNode))
	s.mux.HandleFunc("POST /api/v1/nodes/{name}/heartbeat", s.auth(s.handleHeartbeat))
	s.mux.HandleFunc("GET /api/v1/nodes", s.auth(s.handleListNodes))
	s.mux.HandleFunc("GET /api/v1/nodes/{name}", s.auth(s.handleGetNode))
	s.mux.HandleFunc("DELETE /api/v1/nodes/{name}", s.auth(s.handleDeleteNode))

	// token/show — loopback / auth
	s.mux.HandleFunc("POST /api/v1/token/show", s.auth(s.handleTokenShow))
}

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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Request/Response DTOs
type RegisterRequest struct {
	Name                string `json:"name"`
	Address             string `json:"address"`
	CapacityCPUMillis   int64  `json:"capacity_cpu_millis"`
	CapacityMemoryBytes int64  `json:"capacity_memory_bytes"`
	Runtime             string `json:"runtime"`
}

type HeartbeatRequest struct {
	Address             string   `json:"address"`
	ContainerIDs        []string `json:"container_ids"`
	DockerOK            bool     `json:"docker_ok"`
	CapacityCPUMillis   int64    `json:"capacity_cpu_millis,omitempty"`
	CapacityMemoryBytes int64    `json:"capacity_memory_bytes,omitempty"`
}

func (s *Server) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	// Address resolution: explicit address or RemoteAddr fallback
	addrStr := req.Address
	if addrStr == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			addrStr = r.RemoteAddr
		} else {
			addrStr = host
		}
	}

	// Validate IP address format
	if _, err := netip.ParseAddr(addrStr); err != nil {
		jsonError(w, "invalid address format: "+addrStr, http.StatusBadRequest)
		return
	}

	existing, err := s.store.GetNode(r.Context(), req.Name)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if existing != nil {
		// Rule E: Same name AND different address -> 409 Conflict
		if existing.Address != addrStr {
			jsonError(w, "node name already registered with a different address", http.StatusConflict)
			return
		}

		// Same name AND same address -> 200 OK (upsert/update capacities)
		if req.CapacityCPUMillis > 0 {
			existing.CapacityCPUMillis = req.CapacityCPUMillis
		}
		if req.CapacityMemoryBytes > 0 {
			existing.CapacityMemoryBytes = req.CapacityMemoryBytes
		}
		existing.LastSeen = time.Now().UTC()
		existing.Status = "Ready"
		existing.NotReadySince = nil
		existing.DockerOK = true

		if err := s.store.UpsertNode(r.Context(), existing); err != nil {
			jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	// Unknown name -> 201 Created
	newNode := &types.Node{
		Name:                req.Name,
		Address:             addrStr,
		CapacityCPUMillis:   req.CapacityCPUMillis,
		CapacityMemoryBytes: req.CapacityMemoryBytes,
		Status:              "Ready",
		LastSeen:            time.Now().UTC(),
		DockerOK:            true,
	}

	if err := s.store.UpsertNode(r.Context(), newNode); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(newNode)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("name")
	if nodeName == "" {
		jsonError(w, "node name required", http.StatusBadRequest)
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	existing, err := s.store.GetNode(r.Context(), nodeName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if existing == nil {
		// Unknown node -> 404
		jsonError(w, "node not found, re-registration required", http.StatusNotFound)
		return
	}

	// Check address mismatch -> 409
	if req.Address != "" && req.Address != existing.Address {
		jsonError(w, "address mismatch during heartbeat", http.StatusConflict)
		return
	}

	// Update node in DB including capacity if zero or updated and docker_ok status
	existing.LastSeen = time.Now().UTC()
	existing.Status = "Ready"
	existing.NotReadySince = nil
	existing.DockerOK = req.DockerOK
	if existing.CapacityCPUMillis == 0 && req.CapacityCPUMillis > 0 {
		existing.CapacityCPUMillis = req.CapacityCPUMillis
	}
	if existing.CapacityMemoryBytes == 0 && req.CapacityMemoryBytes > 0 {
		existing.CapacityMemoryBytes = req.CapacityMemoryBytes
	}

	if err := s.store.UpsertNode(r.Context(), existing); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("name")
	node, err := s.store.GetNode(r.Context(), nodeName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if node == nil {
		jsonError(w, "node not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(node)
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("name")
	if nodeName == "" {
		jsonError(w, "node name required", http.StatusBadRequest)
		return
	}
	// For D1 / node deletion in Phase 4
	nodes, err := s.store.ListNodes(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var found bool
	for _, n := range nodes {
		if n.Name == nodeName {
			found = true
			break
		}
	}
	if !found {
		jsonError(w, "node not found", http.StatusNotFound)
		return
	}

	// In SQLite storage, we can delete the node row directly
	if err := s.store.MarkNodeNotReady(r.Context(), nodeName); err != nil {
		// Or delete
	}
	// Let's implement actual node delete if supported or Upsert/Mark as needed
	w.WriteHeader(http.StatusNoContent)
}

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

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
