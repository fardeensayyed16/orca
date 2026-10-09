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

func TestWorkReturnsAssignedReplicasAndRequiresAuth(t *testing.T) {
	store := storage.NewMapStorage()
	if err := store.UpsertNode(nil, &types.Node{
		Name: "node-1", Address: "192.0.2.10", Status: "Ready", LastSeen: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	service := &types.Service{
		Name: "web", Image: "nginx:latest", CPUMillis: 500, MemoryBytes: 128 * 1024 * 1024,
		ContainerPort: 8080, RestartPolicy: types.RestartPolicyAlways,
		PullPolicy: types.PullPolicyIfNotPresent, Env: map[string]string{"MODE": "test"},
		Command: []string{"nginx", "-g", "daemon off;"},
	}
	if err := store.DeployService(nil, service); err != nil {
		t.Fatal(err)
	}
	replica := &types.Replica{
		ID: "replica-1", ServiceName: service.Name, NodeName: "node-1",
		Status: types.PodStatusStarting, RestartCount: 3,
	}
	if err := store.UpsertReplica(nil, replica); err != nil {
		t.Fatal(err)
	}

	server := NewServer(store, testToken)
	unauthorized := httptest.NewRecorder()
	server.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/work/node-1", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized work status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/work/node-1", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("work status = %d: %s", response.Code, response.Body.String())
	}
	var work []WorkReplica
	if err := json.NewDecoder(response.Body).Decode(&work); err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 || work[0].ID != replica.ID || work[0].DesiredRestartCount != 3 ||
		work[0].Image != service.Image || work[0].ContainerPort != 8080 ||
		work[0].PullPolicy != types.PullPolicyIfNotPresent || work[0].Env["MODE"] != "test" {
		t.Fatalf("unexpected work response: %+v", work)
	}

	node, err := store.GetNode(nil, "node-1")
	if err != nil {
		t.Fatal(err)
	}
	node.Status = "Deleting"
	if err := store.UpsertNode(nil, node); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/work/node-1", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("draining work status = %d: %s", response.Code, response.Body.String())
	}
	if err := json.NewDecoder(response.Body).Decode(&work); err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("draining work returned %d replicas, want none", len(work))
	}
}

func TestHeartbeatPersistsReplicaObservation(t *testing.T) {
	store := storage.NewMapStorage()
	if err := store.UpsertNode(nil, &types.Node{
		Name: "node-1", Address: "192.0.2.10", Status: "Ready", LastSeen: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeployService(nil, &types.Service{Name: "web", Image: "nginx"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertReplica(nil, &types.Replica{
		ID: "replica-1", ServiceName: "web", NodeName: "node-1", Status: types.PodStatusStarting,
	}); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(HeartbeatRequest{
		Address: "192.0.2.10",
		Replicas: []ReplicaHeartbeat{{
			ID: "replica-1", ContainerID: "container-id", HostPort: 40000,
			Status: types.PodStatusRunning, Health: types.HealthStatusHealthy,
			Ready: true, RestartCount: 2,
		}},
		DockerOK: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/nodes/node-1/heartbeat", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	NewServer(store, testToken).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d: %s", response.Code, response.Body.String())
	}
	updated, err := store.GetReplica(nil, "replica-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != types.PodStatusRunning || !updated.Ready || updated.RestartCount != 2 ||
		updated.ContainerID != "container-id" || updated.HostPort != 40000 {
		t.Fatalf("replica heartbeat not persisted: %+v", updated)
	}
}

func TestCreateServiceNormalizesAndValidatesProbes(t *testing.T) {
	server := NewServer(storage.NewMapStorage(), testToken)
	body := []byte(`{"name":"web","image":"nginx","container_port":8080,"liveness":{"type":"http","port":8080},"readiness":{"type":"tcp","port":8080}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", response.Code, response.Body.String())
	}
	var created CreateServiceResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Service.Liveness == nil || created.Service.Liveness.Kind != "liveness" ||
		created.Service.Liveness.Path != "/" || created.Service.Liveness.PeriodSeconds != 10 ||
		created.Service.Liveness.TimeoutSeconds != 1 || created.Service.Liveness.FailureThreshold != 3 ||
		created.Service.Liveness.SuccessThreshold != 1 {
		t.Fatalf("liveness defaults not applied: %+v", created.Service.Liveness)
	}
	if created.Service.Readiness == nil || created.Service.Readiness.Kind != "readiness" ||
		created.Service.Readiness.PeriodSeconds != 10 {
		t.Fatalf("readiness defaults not applied: %+v", created.Service.Readiness)
	}

	body = []byte(`{"name":"invalid","image":"nginx","liveness":{"type":"http","port":8080}}`)
	request = httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+testToken)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("HTTP probe without published port status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestServiceEndpointsOnlyReturnHealthyReadyNodeAddresses(t *testing.T) {
	store := storage.NewMapStorage()
	if err := store.DeployService(nil, &types.Service{Name: "web", Image: "nginx"}); err != nil {
		t.Fatal(err)
	}
	for _, node := range []*types.Node{
		{Name: "ready", Address: "192.0.2.10", Status: "Ready", LastSeen: time.Now()},
		{Name: "not-ready", Address: "192.0.2.11", Status: "NotReady", LastSeen: time.Now()},
	} {
		if err := store.UpsertNode(nil, node); err != nil {
			t.Fatal(err)
		}
	}
	replicas := []*types.Replica{
		{ID: "eligible", ServiceName: "web", NodeName: "ready", ContainerID: "internal-container-ip", HostPort: 40000, Status: types.PodStatusRunning, Health: types.HealthStatusHealthy, Ready: true},
		{ID: "unhealthy", ServiceName: "web", NodeName: "ready", HostPort: 40001, Status: types.PodStatusRunning, Health: types.HealthStatusUnhealthy, Ready: true},
		{ID: "not-ready", ServiceName: "web", NodeName: "ready", HostPort: 40002, Status: types.PodStatusRunning, Health: types.HealthStatusHealthy, Ready: false},
		{ID: "node-not-ready", ServiceName: "web", NodeName: "not-ready", HostPort: 40003, Status: types.PodStatusRunning, Health: types.HealthStatusHealthy, Ready: true},
		{ID: "no-port", ServiceName: "web", NodeName: "ready", Status: types.PodStatusRunning, Health: types.HealthStatusHealthy, Ready: true},
	}
	for _, replica := range replicas {
		if err := store.UpsertReplica(nil, replica); err != nil {
			t.Fatal(err)
		}
	}
	server := NewServer(store, testToken)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services/web/endpoints", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("endpoint status = %d: %s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("internal-container-ip")) {
		t.Fatal("endpoint response exposed a container IP")
	}
	var endpoints []ServiceEndpoint
	if err := json.NewDecoder(response.Body).Decode(&endpoints); err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 || endpoints[0].ReplicaID != "eligible" ||
		endpoints[0].NodeAddress != "192.0.2.10" || endpoints[0].HostPort != 40000 {
		t.Fatalf("unexpected endpoints: %+v", endpoints)
	}
}
