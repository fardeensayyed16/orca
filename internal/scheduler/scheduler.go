package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

var scheduleMu sync.Mutex

var ErrNoSuitableNode = errors.New("no suitable ready node")

type nodeUsage struct {
	cpu    int64
	memory int64
}

func ScheduleReplica(ctx context.Context, tx storage.Tx, service *types.Service, replica *types.Replica) error {
	if tx == nil || service == nil || replica == nil {
		return errors.New("scheduler requires a transaction, service, and replica")
	}

	scheduleMu.Lock()
	defer scheduleMu.Unlock()

	nodes, err := tx.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("list scheduler nodes: %w", err)
	}
	replicas, err := tx.ListReplicas(ctx)
	if err != nil {
		return fmt.Errorf("list scheduler replicas: %w", err)
	}
	services, err := tx.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("list scheduler services: %w", err)
	}

	serviceByName := make(map[string]*types.Service, len(services))
	for _, existingService := range services {
		serviceByName[existingService.Name] = existingService
	}
	usageByNode := make(map[string]nodeUsage)
	for _, existing := range replicas {
		if existing.NodeName == "" ||
			(existing.Status != types.PodStatusRunning && existing.Status != types.PodStatusStarting) {
			continue
		}
		existingService := serviceByName[existing.ServiceName]
		if existingService == nil {
			return fmt.Errorf("service %q for replica %q not found", existing.ServiceName, existing.ID)
		}
		usage := usageByNode[existing.NodeName]
		usage.cpu += existingService.CPUMillis
		usage.memory += existingService.MemoryBytes
		usageByNode[existing.NodeName] = usage
	}

	var selected *types.Node
	var selectedFreeCPU float64
	for _, node := range nodes {
		if node.Status != "Ready" || node.CapacityCPUMillis <= 0 || node.CapacityMemoryBytes <= 0 {
			continue
		}
		usage := usageByNode[node.Name]
		if !fits(usage.cpu, service.CPUMillis, node.CapacityCPUMillis) ||
			!fits(usage.memory, service.MemoryBytes, node.CapacityMemoryBytes) {
			continue
		}
		freeCPU := float64(node.CapacityCPUMillis) - float64(usage.cpu)
		if selected == nil || freeCPU > selectedFreeCPU ||
			(freeCPU == selectedFreeCPU && node.Name < selected.Name) {
			selected = node
			selectedFreeCPU = freeCPU
		}
	}
	if selected == nil {
		return ErrNoSuitableNode
	}
	if err := tx.UpdateReplicaNode(ctx, replica.ID, selected.Name); err != nil {
		return fmt.Errorf("bind replica %q to node %q: %w", replica.ID, selected.Name, err)
	}
	replica.NodeName = selected.Name
	replica.Status = types.PodStatusStarting
	return nil
}

func fits(used, requested, capacity int64) bool {
	if used < 0 || requested < 0 || capacity <= 0 {
		return false
	}
	return (float64(used)+float64(requested))*1.05 <= float64(capacity)
}
