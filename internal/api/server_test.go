package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fardeensayyed16/orca/internal/storage"
)

func TestAPIServerPingAndSmoke(t *testing.T) {
	store := storage.NewMapStorage()
	srv := NewServer(store)

	// Test GET /api/v1/ping
	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	if body := w.Body.String(); body != "{\"ok\":true}\n" {
		t.Fatalf("expected {\"ok\":true}, got %q", body)
	}

	// Test GET /api/v1/_test/list
	req = httptest.NewRequest("GET", "/api/v1/_test/list", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for smoke test, got %d", w.Code)
	}
}

