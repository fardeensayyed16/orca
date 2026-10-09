package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/fardeensayyed16/orca/internal/serviceports"
	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestServicePortAllocatedPersistedAndFreedOnDelete(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	firstPort := freeTCPPort(t)
	if firstPort == 65535 {
		t.Fatal("cannot allocate adjacent service ports for test")
	}
	allocator, err := serviceports.NewAllocator(store, strconv.Itoa(firstPort)+"-"+strconv.Itoa(firstPort+1), "127.0.0.1:9001")
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	server := NewServer(store, testToken, allocator)

	create := func(name string) *types.Service {
		t.Helper()
		body, err := json.Marshal(CreateServiceRequest{
			Name: name, Image: "nginx", ContainerPort: 80,
		})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+testToken)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("create service %s returned %d: %s", name, response.Code, response.Body.String())
		}
		var result CreateServiceResponse
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result.Service
	}
	first := create("first")
	if first.ServicePort != firstPort {
		t.Fatalf("first service port = %d, want %d", first.ServicePort, firstPort)
	}
	second := create("second")
	if second.ServicePort != firstPort+1 {
		t.Fatalf("second service port = %d, want %d", second.ServicePort, firstPort+1)
	}
	persisted, err := store.GetService(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil || persisted.ServicePort != firstPort {
		t.Fatalf("persisted service port = %+v, want %d", persisted, firstPort)
	}

	body, err := json.Marshal(CreateServiceRequest{Name: "exhausted", Image: "nginx", ContainerPort: 80})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !bytes.Contains(response.Body.Bytes(), []byte("no free host ports available")) {
		t.Fatalf("exhaustion returned %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/v1/services/first", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete returned %d: %s", response.Code, response.Body.String())
	}
	reused := create("reused")
	if reused.ServicePort != firstPort {
		t.Fatalf("reused service port = %d, want %d", reused.ServicePort, firstPort)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
