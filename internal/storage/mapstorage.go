package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fardeensayyed16/orca/pkg/types"
)

type MapStorage struct {
	mu       sync.RWMutex
	services map[string]*types.Service
	replicas map[string]*types.Replica
	nodes    map[string]*types.Node
	metrics  []*types.Metrics
	token    string
}

type mapTx struct {
	store *MapStorage
}

func (m *mapTx) Commit() error   { return nil }
func (m *mapTx) Rollback() error { return nil }
func (m *mapTx) ListServices(ctx context.Context) ([]*types.Service, error) {
	return m.store.ListServices(ctx)
}
func (m *mapTx) ListNodes(ctx context.Context) ([]*types.Node, error) {
	return m.store.ListNodes(ctx)
}
func (m *mapTx) ListReplicas(ctx context.Context) ([]*types.Replica, error) {
	return m.store.ListReplicas(ctx)
}
func (m *mapTx) UpdateReplicaNode(ctx context.Context, id, nodeName string) error {
	return m.store.UpdateReplicaNode(ctx, id, nodeName)
}

func NewMapStorage() *MapStorage {
	return &MapStorage{
		services: make(map[string]*types.Service),
		replicas: make(map[string]*types.Replica),
		nodes:    make(map[string]*types.Node),
		metrics:  make([]*types.Metrics, 0),
	}
}

func (m *MapStorage) Close() error {
	return nil
}

func (m *MapStorage) WithTx(ctx context.Context, fn func(tx Tx) error) error {
	return fn(&mapTx{store: m})
}

func (m *MapStorage) DeployService(ctx context.Context, svc *types.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[svc.Name] = svc
	return nil
}

func (m *MapStorage) ListServices(ctx context.Context) ([]*types.Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*types.Service
	for _, s := range m.services {
		list = append(list, s)
	}
	return list, nil
}

func (m *MapStorage) GetService(ctx context.Context, name string) (*types.Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	svc, ok := m.services[name]
	if !ok {
		return nil, nil
	}
	return svc, nil
}

func (m *MapStorage) DeleteService(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.services, name)
	return nil
}

func (m *MapStorage) UpsertReplica(ctx context.Context, r *types.Replica) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replicas[r.ID] = r
	return nil
}

func (m *MapStorage) ListReplicas(ctx context.Context) ([]*types.Replica, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*types.Replica
	for _, r := range m.replicas {
		list = append(list, r)
	}
	return list, nil
}

func (m *MapStorage) ListReplicasByService(ctx context.Context, serviceName string) ([]*types.Replica, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*types.Replica
	for _, r := range m.replicas {
		if r.ServiceName == serviceName {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *MapStorage) UpdateReplicaNode(ctx context.Context, id, nodeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.replicas[id]
	if !ok || r.Status != types.PodStatusPending {
		return fmt.Errorf("replica %q not found or not pending", id)
	}
	r.NodeName = nodeName
	r.Status = types.PodStatusStarting
	return nil
}

func (m *MapStorage) GetReplica(ctx context.Context, id string) (*types.Replica, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.replicas[id]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *MapStorage) DeleteReplica(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.replicas, id)
	return nil
}

func (m *MapStorage) UpsertNode(ctx context.Context, node *types.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.nodes[node.Name]; ok && existing.Status == "Deleting" && node.Status != "Deleting" {
		node.Status = "Deleting"
		if node.NotReadySince == nil {
			node.NotReadySince = existing.NotReadySince
		}
	}
	m.nodes[node.Name] = node
	return nil
}

func (m *MapStorage) DeleteNode(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodes, name)
	return nil
}

func (m *MapStorage) ListNodes(ctx context.Context) ([]*types.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*types.Node
	for _, n := range m.nodes {
		list = append(list, n)
	}
	return list, nil
}

func (m *MapStorage) GetNode(ctx context.Context, name string) (*types.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[name]
	if !ok {
		return nil, nil
	}
	return n, nil
}

func (m *MapStorage) MarkNodeReady(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.nodes[name]; ok {
		n.Status = "Ready"
		n.LastSeen = time.Now().UTC()
		n.NotReadySince = nil
	}
	return nil
}

func (m *MapStorage) MarkNodeNotReady(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.nodes[name]; ok {
		n.Status = "NotReady"
		now := time.Now().UTC()
		if n.NotReadySince == nil {
			n.NotReadySince = &now
		}
	}
	return nil
}

func (m *MapStorage) RecordHeartbeat(ctx context.Context, name string, dockerOK bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.nodes[name]; ok {
		if n.Status != "Deleting" {
			n.Status = "Ready"
			n.NotReadySince = nil
		}
		n.LastSeen = time.Now().UTC()
		n.DockerOK = dockerOK
	}
	return nil
}

func (m *MapStorage) UpdateReplicaHeartbeat(ctx context.Context, nodeName string, replica *types.Replica) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.replicas[replica.ID]
	if !ok || existing.NodeName != nodeName {
		return fmt.Errorf("replica %q is not assigned to node %q", replica.ID, nodeName)
	}
	existing.ContainerID = replica.ContainerID
	existing.HostPort = replica.HostPort
	existing.Status = replica.Status
	existing.Health = replica.Health
	existing.Ready = replica.Ready
	existing.RestartCount = replica.RestartCount
	existing.ExitCode = replica.ExitCode
	existing.Message = replica.Message
	return nil
}

func (m *MapStorage) InsertMetric(ctx context.Context, metric *types.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, metric)
	return nil
}

func (m *MapStorage) PruneOldMetrics(ctx context.Context, olderThan time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().UTC().Add(-olderThan)
	var kept []*types.Metrics
	for _, met := range m.metrics {
		if met.Timestamp.After(cutoff) {
			kept = append(kept, met)
		}
	}
	m.metrics = kept
	return nil
}

func (m *MapStorage) GetToken(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token, nil
}

func (m *MapStorage) SetToken(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
	return nil
}
