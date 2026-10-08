package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fardeensayyed16/orca/internal/storage"
)

const testToken = "test-token-abc123"

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store := storage.NewMapStorage()
	return NewServer(store, testToken)
}

func TestPing_NoAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ping: want 200 got %d", w.Code)
	}
}

func TestSmokeList_NoAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/_test/list", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("_test/list without auth: want 200 got %d", w.Code)
	}
}

func TestSmokeList_WrongToken_Still200(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/_test/list", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// _test/* is unauthenticated — wrong token still gets 200
	if w.Code != http.StatusOK {
		t.Fatalf("_test/list with wrong token: want 200 got %d", w.Code)
	}
}

func TestNodes_NoToken_Returns401(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("nodes without token: want 401 got %d", w.Code)
	}
}

func TestNodes_WrongToken_Returns401(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("nodes with wrong token: want 401 got %d", w.Code)
	}
}

func TestNodes_CorrectToken_Returns200(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nodes with correct token: want 200 got %d", w.Code)
	}
}
