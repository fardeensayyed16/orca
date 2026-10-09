package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestDeleteNodeStartsDrainAndAcceptsRunningReplicas(t *testing.T) {
	store := storage.NewMapStorage()
	if err := store.UpsertNode(nil, &types.Node{
		Name: "node-1", Address: "192.0.2.10", Status: "Ready", LastSeen: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertReplica(nil, &types.Replica{
		ID: "replica-1", ServiceName: "web", NodeName: "node-1",
		Status: types.PodStatusRunning,
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/node-1", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	NewServer(store, testToken).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("delete status = %d, want %d", response.Code, http.StatusAccepted)
	}
	node, err := store.GetNode(nil, "node-1")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil || node.Status != "Deleting" || node.NotReadySince == nil {
		t.Fatalf("node drain was not persisted: %+v", node)
	}
	heartbeat := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/nodes/node-1/heartbeat",
		strings.NewReader(`{"address":"192.0.2.10","docker_ok":true,"replicas":[]}`),
	)
	heartbeat.Header.Set("Authorization", "Bearer "+testToken)
	heartbeatResponse := httptest.NewRecorder()
	NewServer(store, testToken).ServeHTTP(heartbeatResponse, heartbeat)
	if heartbeatResponse.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d: %s", heartbeatResponse.Code, heartbeatResponse.Body.String())
	}
	node, err = store.GetNode(nil, "node-1")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil || node.Status != "Deleting" || node.NotReadySince == nil {
		t.Fatalf("worker heartbeat cancelled drain: %+v", node)
	}
}

func TestDeleteNodeReturnsNoContentWhenAlreadyCleanAndNotFoundForUnknown(t *testing.T) {
	store := storage.NewMapStorage()
	if err := store.UpsertNode(nil, &types.Node{
		Name: "clean-node", Address: "192.0.2.10", Status: "Ready", LastSeen: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	server := NewServer(store, testToken)

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/clean-node", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("clean delete status = %d, want %d", response.Code, http.StatusNoContent)
	}
	node, err := store.GetNode(nil, "clean-node")
	if err != nil {
		t.Fatal(err)
	}
	if node != nil {
		t.Fatalf("clean node row was not removed: %+v", node)
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/missing", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown delete status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
