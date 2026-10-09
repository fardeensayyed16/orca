package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestScheduleReplicaBalancesNodesDeterministically(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	if err := store.DeployService(ctx, &types.Service{
		Name: "web", Image: "nginx", Replicas: 4, CPUMillis: 1000, MemoryBytes: 512,
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"node-b", "node-a"} {
		if err := store.UpsertNode(ctx, &types.Node{
			Name: name, Status: "Ready", CapacityCPUMillis: 2100, CapacityMemoryBytes: 2048,
		}); err != nil {
			t.Fatal(err)
		}
	}

	counts := map[string]int{}
	for i := 0; i < 4; i++ {
		replica := &types.Replica{
			ID: string(rune('a' + i)), ServiceName: "web", Status: types.PodStatusPending,
		}
		if err := store.UpsertReplica(ctx, replica); err != nil {
			t.Fatal(err)
		}
		if err := store.WithTx(ctx, func(tx storage.Tx) error {
			return ScheduleReplica(ctx, tx, &types.Service{
				Name: "web", CPUMillis: 1000, MemoryBytes: 512,
			}, replica)
		}); err != nil {
			t.Fatal(err)
		}
		if replica.Status != types.PodStatusStarting {
			t.Fatalf("replica status = %q, want Starting", replica.Status)
		}
		if i == 0 && replica.NodeName != "node-a" {
			t.Fatalf("first tie was placed on %q, want node-a", replica.NodeName)
		}
		counts[replica.NodeName]++
	}
	if counts["node-a"] != 2 || counts["node-b"] != 2 {
		t.Fatalf("placement counts = %#v, want 2 per node", counts)
	}
}

func TestScheduleReplicaAccountsForOtherServiceAndSafetyMargin(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	for _, service := range []*types.Service{
		{Name: "existing", Image: "old", Replicas: 1, CPUMillis: 500, MemoryBytes: 400},
		{Name: "new", Image: "new", Replicas: 1, CPUMillis: 500, MemoryBytes: 400},
	} {
		if err := store.DeployService(ctx, service); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.UpsertNode(ctx, &types.Node{
		Name: "node-a", Status: "Ready", CapacityCPUMillis: 1000, CapacityMemoryBytes: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertReplica(ctx, &types.Replica{
		ID: "running", ServiceName: "existing", NodeName: "node-a", Status: types.PodStatusRunning,
	}); err != nil {
		t.Fatal(err)
	}
	replica := &types.Replica{ID: "pending", ServiceName: "new", Status: types.PodStatusPending}
	if err := store.UpsertReplica(ctx, replica); err != nil {
		t.Fatal(err)
	}

	err := store.WithTx(ctx, func(tx storage.Tx) error {
		return ScheduleReplica(ctx, tx, &types.Service{
			Name: "new", CPUMillis: 500, MemoryBytes: 400,
		}, replica)
	})
	if !errors.Is(err, ErrNoSuitableNode) {
		t.Fatalf("ScheduleReplica error = %v, want ErrNoSuitableNode", err)
	}
	if replica.NodeName != "" || replica.Status != types.PodStatusPending {
		t.Fatalf("unscheduled replica was modified: %+v", replica)
	}
}

func TestScheduleReplicaWithSQLiteTransaction(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := &types.Service{
		Name: "web", Image: "nginx", Replicas: 1, CPUMillis: 500, MemoryBytes: 512,
	}
	if err := store.DeployService(ctx, service); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNode(ctx, &types.Node{
		Name: "node-a", Status: "Ready", CapacityCPUMillis: 1000, CapacityMemoryBytes: 2048,
	}); err != nil {
		t.Fatal(err)
	}
	replica := &types.Replica{ID: "replica-1", ServiceName: "web", Status: types.PodStatusPending}
	if err := store.UpsertReplica(ctx, replica); err != nil {
		t.Fatal(err)
	}
	if err := store.WithTx(ctx, func(tx storage.Tx) error {
		return ScheduleReplica(ctx, tx, service, replica)
	}); err != nil {
		t.Fatal(err)
	}
	persisted, err := store.GetReplica(ctx, replica.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.NodeName != "node-a" || persisted.Status != types.PodStatusStarting {
		t.Fatalf("persisted replica = %+v, want bound Starting replica", persisted)
	}
	serviceReplicas, err := store.ListReplicasByService(ctx, service.Name)
	if err != nil || len(serviceReplicas) != 1 {
		t.Fatalf("ListReplicasByService returned %d replicas, err %v", len(serviceReplicas), err)
	}
}
