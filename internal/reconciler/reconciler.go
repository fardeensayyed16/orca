package reconciler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fardeensayyed16/orca/internal/scheduler"
	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

const (
	defaultReconcileInterval = 15 * time.Second
	orphanThreshold          = 30 * time.Second
	nodeDeleteGracePeriod    = 5 * time.Minute
)

type HAReconciler struct {
	store    storage.Storage
	interval time.Duration
}

func NewHAReconciler(store storage.Storage, interval time.Duration) *HAReconciler {
	if interval <= 0 {
		interval = defaultReconcileInterval
	}
	return &HAReconciler{store: store, interval: interval}
}

func (r *HAReconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil && ctx.Err() == nil {
				log.Printf("HA reconciliation error: %v", err)
			}
		}
	}
}

func (r *HAReconciler) Reconcile(ctx context.Context) error {
	nodes, err := r.store.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}
	replicas, err := r.store.ListReplicas(ctx)
	if err != nil {
		return fmt.Errorf("list replicas: %w", err)
	}
	services, err := r.store.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	nodesByName := make(map[string]*types.Node, len(nodes))
	for _, node := range nodes {
		nodesByName[node.Name] = node
	}
	servicesByName := make(map[string]*types.Service, len(services))
	for _, service := range services {
		servicesByName[service.Name] = service
	}

	if err := r.ensureDesiredReplicas(ctx, services, replicas); err != nil {
		return err
	}
	for _, replica := range replicas {
		if replica.NodeName == "" {
			if replica.Status == types.PodStatusPending {
				r.schedule(ctx, replica, servicesByName[replica.ServiceName], "", false)
			}
			continue
		}

		node := nodesByName[replica.NodeName]
		if node == nil {
			log.Printf("Replica %s refers to missing node %s; treating it as an orphan", replica.ID, replica.NodeName)
			if isTerminal(replica.Status) {
				replica.NodeName = ""
				replica.ContainerID = ""
				replica.HostPort = 0
				if err := r.store.UpsertReplica(ctx, replica); err != nil {
					return fmt.Errorf("detach terminal replica %q from missing node: %w", replica.ID, err)
				}
				continue
			}
			r.reschedule(ctx, replica, servicesByName[replica.ServiceName], replica.NodeName)
			continue
		}

		if node.Status == "Deleting" {
			if isTerminal(replica.Status) {
				replica.NodeName = ""
				replica.ContainerID = ""
				replica.HostPort = 0
				if err := r.store.UpsertReplica(ctx, replica); err != nil {
					return fmt.Errorf("detach terminal replica %q from deleting node: %w", replica.ID, err)
				}
				continue
			}
			r.reschedule(ctx, replica, servicesByName[replica.ServiceName], node.Name)
			continue
		}

		if replica.Status == types.PodStatusPending && node.Status == "Ready" {
			r.schedule(ctx, replica, servicesByName[replica.ServiceName], node.Name, true)
			continue
		}

		if node.Status == "NotReady" {
			if !isTerminal(replica.Status) &&
				(replica.Status != types.PodStatusUnknown || replica.Ready || replica.Health != types.HealthStatusUnknown) {
				replica.Status = types.PodStatusUnknown
				replica.Health = types.HealthStatusUnknown
				replica.Ready = false
				if err := r.store.UpsertReplica(ctx, replica); err != nil {
					return fmt.Errorf("mark replica %q unknown on node %q: %w", replica.ID, node.Name, err)
				}
			}
			if ageSinceNotReady(node, time.Now().UTC()) > orphanThreshold && !isTerminal(replica.Status) {
				r.reschedule(ctx, replica, servicesByName[replica.ServiceName], node.Name)
			}
		}
	}

	// Delete only nodes explicitly marked for deletion, after the grace period
	// and after every replica has either moved or been detached.
	for _, node := range nodes {
		if node.Status != "Deleting" || ageSinceNotReady(node, time.Now().UTC()) <= nodeDeleteGracePeriod {
			continue
		}
		hasReplicas := false
		for _, replica := range replicas {
			if replica.NodeName == node.Name {
				hasReplicas = true
				break
			}
		}
		if !hasReplicas {
			if err := r.store.DeleteNode(ctx, node.Name); err != nil {
				return fmt.Errorf("delete drained node %q: %w", node.Name, err)
			}
		}
	}
	return nil
}

func (r *HAReconciler) ensureDesiredReplicas(ctx context.Context, services []*types.Service, replicas []*types.Replica) error {
	counts := make(map[string]int, len(services))
	for _, replica := range replicas {
		counts[replica.ServiceName]++
	}
	for _, service := range services {
		for counts[service.Name] < service.Replicas {
			id, err := newReplicaID()
			if err != nil {
				return fmt.Errorf("create replica ID for service %q: %w", service.Name, err)
			}
			replica := &types.Replica{
				ID: id, ServiceName: service.Name, Status: types.PodStatusPending,
				Health: types.HealthStatusUnknown, CreatedAt: time.Now().UTC(),
			}
			if err := r.store.UpsertReplica(ctx, replica); err != nil {
				return fmt.Errorf("create desired replica for service %q: %w", service.Name, err)
			}
			log.Printf("Warning: created missing replica %s for service %s in Pending", id, service.Name)
			counts[service.Name]++
		}
	}
	return nil
}

func (r *HAReconciler) reschedule(ctx context.Context, replica *types.Replica, service *types.Service, deadNode string) {
	replica.NodeName = deadNode
	replica.ContainerID = ""
	replica.HostPort = 0
	replica.Status = types.PodStatusPending
	replica.Health = types.HealthStatusUnknown
	replica.Ready = false
	if err := r.store.UpsertReplica(ctx, replica); err != nil {
		log.Printf("Failed to prepare orphan replica %s for rescheduling: %v", replica.ID, err)
		return
	}
	r.schedule(ctx, replica, service, deadNode, true)
}

func (r *HAReconciler) schedule(ctx context.Context, replica *types.Replica, service *types.Service, deadNode string, rescheduled bool) {
	if service == nil {
		log.Printf("Cannot schedule replica %s: service %s not found", replica.ID, replica.ServiceName)
		return
	}
	err := r.store.WithTx(ctx, func(tx storage.Tx) error {
		return scheduler.ScheduleReplica(ctx, tx, service, replica)
	})
	if err != nil {
		if errors.Is(err, scheduler.ErrNoSuitableNode) {
			log.Printf("Warning: no suitable node for Pending replica %s", replica.ID)
			return
		}
		log.Printf("Failed to schedule replica %s: %v", replica.ID, err)
		return
	}
	if rescheduled && replica.NodeName != deadNode {
		replica.RescheduleCount++
		now := time.Now().UTC()
		replica.LastRescheduledAt = &now
		log.Printf("rescheduled replica %s from %s to %s", replica.ID, deadNode, replica.NodeName)
	}
	if err := r.store.UpsertReplica(ctx, replica); err != nil {
		log.Printf("Failed to persist scheduled replica %s: %v", replica.ID, err)
	}
}

func ageSinceNotReady(node *types.Node, now time.Time) time.Duration {
	since := node.NotReadySince
	if since == nil {
		since = &node.LastSeen
	}
	if since.IsZero() {
		return 0
	}
	return now.Sub(*since)
}

func isTerminal(status types.PodStatus) bool {
	return status == types.PodStatusCompleted || status == types.PodStatusFailed
}

func newReplicaID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
