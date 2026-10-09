package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestReconcileReschedulesOrphanAndIncrementsCount(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	service := &types.Service{
		Name: "web", Image: "nginx", Replicas: 1,
		CPUMillis: 100, MemoryBytes: 1024,
	}
	if err := store.DeployService(ctx, service); err != nil {
		t.Fatal(err)
	}
	deadSince := time.Now().UTC().Add(-31 * time.Second)
	for _, node := range []*types.Node{
		{Name: "dead", Address: "192.0.2.1", Status: "NotReady", NotReadySince: &deadSince, CapacityCPUMillis: 1000, CapacityMemoryBytes: 1024 * 1024},
		{Name: "live", Address: "192.0.2.2", Status: "Ready", CapacityCPUMillis: 1000, CapacityMemoryBytes: 1024 * 1024},
	} {
		if err := store.UpsertNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	replica := &types.Replica{
		ID: "replica-1", ServiceName: service.Name, NodeName: "dead",
		ContainerID: "old-container", HostPort: 40000,
		Status: types.PodStatusRunning, Health: types.HealthStatusHealthy, Ready: true,
	}
	if err := store.UpsertReplica(ctx, replica); err != nil {
		t.Fatal(err)
	}

	if err := NewHAReconciler(store, time.Second).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetReplica(ctx, replica.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeName != "live" || got.Status != types.PodStatusStarting ||
		got.ContainerID != "" || got.HostPort != 0 || got.Ready ||
		got.Health != types.HealthStatusUnknown || got.RescheduleCount != 1 ||
		got.LastRescheduledAt == nil {
		t.Fatalf("replica was not fully rescheduled: %+v", got)
	}
}

func TestReconcileMarksNotReadyAndKeepsPendingWithoutCapacity(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	service := &types.Service{
		Name: "web", Image: "nginx", Replicas: 1,
		CPUMillis: 100, MemoryBytes: 1024,
	}
	if err := store.DeployService(ctx, service); err != nil {
		t.Fatal(err)
	}
	deadSince := time.Now().UTC().Add(-31 * time.Second)
	if err := store.UpsertNode(ctx, &types.Node{
		Name: "dead", Address: "192.0.2.1", Status: "NotReady",
		NotReadySince: &deadSince, CapacityCPUMillis: 1000, CapacityMemoryBytes: 1024 * 1024,
	}); err != nil {
		t.Fatal(err)
	}
	replica := &types.Replica{
		ID: "replica-1", ServiceName: service.Name, NodeName: "dead",
		Status: types.PodStatusRunning, Health: types.HealthStatusHealthy, Ready: true,
	}
	if err := store.UpsertReplica(ctx, replica); err != nil {
		t.Fatal(err)
	}

	if err := NewHAReconciler(store, time.Second).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetReplica(ctx, replica.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeName != "dead" || got.Status != types.PodStatusPending || got.Ready ||
		got.Health != types.HealthStatusUnknown || got.RescheduleCount != 0 {
		t.Fatalf("replica without a target node is not pending: %+v", got)
	}
	if err := store.UpsertNode(ctx, &types.Node{
		Name: "live", Address: "192.0.2.2", Status: "Ready",
		CapacityCPUMillis: 1000, CapacityMemoryBytes: 1024 * 1024,
	}); err != nil {
		t.Fatal(err)
	}
	if err := NewHAReconciler(store, time.Second).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetReplica(ctx, replica.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeName != "live" || got.Status != types.PodStatusStarting || got.RescheduleCount != 1 {
		t.Fatalf("pending orphan did not complete reschedule: %+v", got)
	}
}

func TestReconcileDrainsDeletingNodeAndRemovesAfterGrace(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	service := &types.Service{
		Name: "web", Image: "nginx", Replicas: 1,
		CPUMillis: 100, MemoryBytes: 1024,
	}
	if err := store.DeployService(ctx, service); err != nil {
		t.Fatal(err)
	}
	deletingSince := time.Now().UTC().Add(-6 * time.Minute)
	for _, node := range []*types.Node{
		{Name: "draining", Address: "192.0.2.1", Status: "Deleting", NotReadySince: &deletingSince, CapacityCPUMillis: 1000, CapacityMemoryBytes: 1024 * 1024},
		{Name: "live", Address: "192.0.2.2", Status: "Ready", CapacityCPUMillis: 1000, CapacityMemoryBytes: 1024 * 1024},
	} {
		if err := store.UpsertNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	replica := &types.Replica{
		ID: "replica-1", ServiceName: service.Name, NodeName: "draining",
		Status: types.PodStatusRunning,
	}
	if err := store.UpsertReplica(ctx, replica); err != nil {
		t.Fatal(err)
	}

	if err := NewHAReconciler(store, time.Second).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	node, err := store.GetNode(ctx, "draining")
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetReplica(ctx, replica.ID)
	if err != nil {
		t.Fatal(err)
	}
	if node != nil || got.NodeName != "live" {
		t.Fatalf("deleting node was not drained and removed: node=%+v replica=%+v", node, got)
	}
}

func TestReconcileCreatesMissingDesiredReplicas(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	if err := store.DeployService(ctx, &types.Service{Name: "web", Image: "nginx", Replicas: 2}); err != nil {
		t.Fatal(err)
	}
	if err := NewHAReconciler(store, time.Second).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	replicas, err := store.ListReplicas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(replicas) != 2 {
		t.Fatalf("created %d replicas, want 2", len(replicas))
	}
	for _, replica := range replicas {
		if replica.Status != types.PodStatusPending {
			t.Fatalf("new replica status = %q, want Pending", replica.Status)
		}
	}
}

func TestReconcileScalesDownReplicasByPriority(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	if err := store.DeployService(ctx, &types.Service{Name: "web", Image: "nginx", Replicas: 3}); err != nil {
		t.Fatal(err)
	}
	oldStarted := time.Now().UTC().Add(-time.Hour)
	newStarted := time.Now().UTC()
	replicas := []*types.Replica{
		{ID: "pending", ServiceName: "web", Status: types.PodStatusPending},
		{ID: "not-ready-old", ServiceName: "web", Status: types.PodStatusRunning, StartedAt: &oldStarted},
		{ID: "not-ready-new", ServiceName: "web", Status: types.PodStatusRunning, StartedAt: &newStarted},
		{ID: "ready-old", ServiceName: "web", Status: types.PodStatusRunning, Ready: true, StartedAt: &oldStarted},
		{ID: "ready-new", ServiceName: "web", Status: types.PodStatusRunning, Ready: true, StartedAt: &newStarted},
	}
	for _, replica := range replicas {
		if err := store.UpsertReplica(ctx, replica); err != nil {
			t.Fatal(err)
		}
	}

	if err := NewHAReconciler(store, time.Second).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListReplicas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("replica count = %d, want 3", len(got))
	}
	for _, id := range []string{"pending", "not-ready-new"} {
		replica, err := store.GetReplica(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if replica != nil {
			t.Errorf("scale-down victim %q still exists", id)
		}
	}
	for _, id := range []string{"not-ready-old", "ready-old", "ready-new"} {
		replica, err := store.GetReplica(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if replica == nil {
			t.Errorf("expected replica %q to remain", id)
		}
	}
}
