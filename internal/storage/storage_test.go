package storage

import (
	"context"
	"testing"
	"time"

	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestStorageImplementations(t *testing.T) {
	t.Run("MapStorage", func(t *testing.T) {
		store := NewMapStorage()
		testStorage(t, store)
	})

	t.Run("SQLiteStorage", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewSQLiteStorage(tmpDir)
		if err != nil {
			t.Fatalf("failed to create sqlite storage: %v", err)
		}
		defer store.Close()
		testStorage(t, store)
	})
}

func testStorage(t *testing.T, store Storage) {
	ctx := context.Background()

	// Token test
	if err := store.SetToken(ctx, "test-token-123"); err != nil {
		t.Fatalf("SetToken error: %v", err)
	}
	tok, err := store.GetToken(ctx)
	if err != nil || tok != "test-token-123" {
		t.Fatalf("GetToken mismatch: got %q, err %v", tok, err)
	}

	// Service test
	svc := &types.Service{
		Name:          "web",
		Image:         "nginx:alpine",
		Replicas:      2,
		ContainerPort: 80,
		ServicePort:   30080,
		CPUMillis:     500,
		MemoryBytes:   256 * 1024 * 1024,
		RestartPolicy: types.RestartPolicyAlways,
		PullPolicy:    types.PullPolicyIfNotPresent,
	}
	if err := store.DeployService(ctx, svc); err != nil {
		t.Fatalf("DeployService error: %v", err)
	}

	retSvc, err := store.GetService(ctx, "web")
	if err != nil || retSvc == nil || retSvc.Image != "nginx:alpine" {
		t.Fatalf("GetService error or mismatch: %+v, err: %v", retSvc, err)
	}

	// Node test
	node := &types.Node{
		Name:                "node-1",
		Address:             "192.168.1.20",
		CapacityCPUMillis:   8000,
		CapacityMemoryBytes: 8 * 1024 * 1024 * 1024,
		Status:              "Ready",
		DockerOK:            true,
	}
	if err := store.UpsertNode(ctx, node); err != nil {
		t.Fatalf("UpsertNode error: %v", err)
	}

	// Replica test
	rep := &types.Replica{
		ID:          "web-rep-1",
		ServiceName: "web",
		NodeName:    "node-1",
		HostPort:    40001,
		Status:      types.PodStatusRunning,
		Health:      types.HealthStatusHealthy,
		Ready:       true,
	}
	if err := store.UpsertReplica(ctx, rep); err != nil {
		t.Fatalf("UpsertReplica error: %v", err)
	}

	reps, err := store.ListReplicas(ctx)
	if err != nil || len(reps) != 1 {
		t.Fatalf("ListReplicas failed: len %d, err %v", len(reps), err)
	}

	// Prune metrics test
	m := &types.Metrics{
		ReplicaID:   "web-rep-1",
		Timestamp:   time.Now().UTC().Add(-2 * time.Hour),
		CPUCores:    0.5,
		MemoryBytes: 1000,
	}
	if err := store.InsertMetric(ctx, m); err != nil {
		t.Fatalf("InsertMetric error: %v", err)
	}
	if err := store.PruneOldMetrics(ctx, 1*time.Hour); err != nil {
		t.Fatalf("PruneOldMetrics error: %v", err)
	}
}

