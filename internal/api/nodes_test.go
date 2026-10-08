package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

const testTokenPhase4 = "test-token-phase4"

func newTestServerPhase4(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	store := storage.NewMapStorage()
	srv := NewServer(store, testTokenPhase4)
	return srv, store
}

func TestRegisterNode_Success(t *testing.T) {
	srv, _ := newTestServerPhase4(t)

	reqBody := RegisterRequest{
		Name:                "node-1",
		Address:             "192.168.1.20",
		CapacityCPUMillis:   8000,
		CapacityMemoryBytes: 8589934592,
		Runtime:             "docker",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/nodes/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testTokenPhase4)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterNode_InvalidAddress(t *testing.T) {
	srv, _ := newTestServerPhase4(t)

	reqBody := RegisterRequest{
		Name:                "node-1",
		Address:             "invalid-ip-address",
		CapacityCPUMillis:   8000,
		CapacityMemoryBytes: 8589934592,
		Runtime:             "docker",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/nodes/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testTokenPhase4)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestRegisterNode_RuleE_Conflict(t *testing.T) {
	srv, store := newTestServerPhase4(t)

	// Pre-register node-1 at 192.168.1.20
	_ = store.UpsertNode(nil, &types.Node{
		Name:                "node-1",
		Address:             "192.168.1.20",
		CapacityCPUMillis:   8000,
		CapacityMemoryBytes: 8589934592,
		Status:              "Ready",
		LastSeen:            time.Now(),
		DockerOK:            true,
	})

	// Register node-1 with different address 192.168.1.30 -> 409 Conflict
	reqBody := RegisterRequest{
		Name:                "node-1",
		Address:             "192.168.1.30",
		CapacityCPUMillis:   8000,
		CapacityMemoryBytes: 8589934592,
		Runtime:             "docker",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/nodes/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testTokenPhase4)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", w.Code)
	}
}

func TestHeartbeat_NotFound(t *testing.T) {
	srv, _ := newTestServerPhase4(t)

	reqBody := HeartbeatRequest{
		Address:  "192.168.1.20",
		DockerOK: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/nodes/unknown-node/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testTokenPhase4)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestHeartbeat_AddressMismatchConflict(t *testing.T) {
	srv, store := newTestServerPhase4(t)

	_ = store.UpsertNode(nil, &types.Node{
		Name:     "node-1",
		Address:  "192.168.1.20",
		Status:   "Ready",
		LastSeen: time.Now(),
		DockerOK: true,
	})

	reqBody := HeartbeatRequest{
		Address:  "192.168.1.99",
		DockerOK: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/nodes/node-1/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testTokenPhase4)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on heartbeat address mismatch, got %d", w.Code)
	}
}

