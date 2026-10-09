package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestCreateServiceAndListPods(t *testing.T) {
	store := storage.NewMapStorage()
	srv := NewServer(store, testToken)
	body := []byte(`{"name":"web","image":"nginx","replicas":2}`)

	unauthenticated := httptest.NewRecorder()
	srv.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(body)))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("POST without token: got %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST services: got %d: %s", response.Code, response.Body.String())
	}
	var created CreateServiceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Service.CPUMillis != 1000 || created.Service.MemoryBytes != 536870912 {
		t.Fatalf("defaults = CPU %d, memory %d", created.Service.CPUMillis, created.Service.MemoryBytes)
	}
	if len(created.Replicas) != 2 || created.Replicas[0].Status != types.PodStatusPending {
		t.Fatalf("created replicas = %+v, want two pending replicas without nodes", created.Replicas)
	}

	podsReq := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
	podsReq.Header.Set("Authorization", "Bearer "+testToken)
	podsResponse := httptest.NewRecorder()
	srv.ServeHTTP(podsResponse, podsReq)
	var pods []*types.Replica
	if err := json.Unmarshal(podsResponse.Body.Bytes(), &pods); err != nil {
		t.Fatal(err)
	}
	if podsResponse.Code != http.StatusOK || len(pods) != 2 {
		t.Fatalf("GET pods: status %d, count %d", podsResponse.Code, len(pods))
	}

	servicesReq := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	servicesReq.Header.Set("Authorization", "Bearer "+testToken)
	servicesResponse := httptest.NewRecorder()
	srv.ServeHTTP(servicesResponse, servicesReq)
	var services []ServiceWithReplicaSummary
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &services); err != nil {
		t.Fatal(err)
	}
	if servicesResponse.Code != http.StatusOK || len(services) != 1 ||
		services[0].ReplicaSummary.Desired != 2 || services[0].ReplicaSummary.Pending != 2 {
		t.Fatalf("GET services returned status %d and data %+v", servicesResponse.Code, services)
	}
}
