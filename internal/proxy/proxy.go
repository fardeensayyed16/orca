package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

type Backend struct {
	Address  string
	HostPort int
}

type ProxyManager struct {
	mu        sync.Mutex
	listeners map[int]net.Listener
	store     storage.Storage
	bindAddr  string
}

func NewProxyManager(store storage.Storage, bindAddr string) *ProxyManager {
	return &ProxyManager{
		listeners: make(map[int]net.Listener),
		store:     store,
		bindAddr:  bindAddr,
	}
}

// RestoreListeners sweeps existing Ready endpoints in SQLite and restores TCP listeners per Rule C2
func (pm *ProxyManager) RestoreListeners(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	services, err := pm.store.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to list services for proxy restore: %w", err)
	}

	replicas, err := pm.store.ListReplicas(ctx)
	if err != nil {
		return fmt.Errorf("failed to list replicas for proxy restore: %w", err)
	}

	nodes, err := pm.store.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list nodes for proxy restore: %w", err)
	}

	nodeMap := make(map[string]*types.Node)
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	// Find active service ports with ready endpoints
	for _, svc := range services {
		if svc.ServicePort <= 0 {
			continue
		}

		var activeBackends []Backend
		for _, rep := range replicas {
			if rep.ServiceName == svc.Name && rep.Status == types.PodStatusRunning && rep.Health == types.HealthStatusHealthy && rep.Ready {
				if node, ok := nodeMap[rep.NodeName]; ok && node.Status == "Ready" {
					activeBackends = append(activeBackends, Backend{
						Address:  node.Address,
						HostPort: rep.HostPort,
					})
				}
			}
		}

		if len(activeBackends) > 0 {
			if _, exists := pm.listeners[svc.ServicePort]; !exists {
				if err := pm.startListenerLocked(ctx, svc.Name, svc.ServicePort); err != nil {
					log.Printf("Failed to restore proxy listener on port %d for service %s: %v", svc.ServicePort, svc.Name, err)
				}
			}
		}
	}

	return nil
}

func (pm *ProxyManager) startListenerLocked(ctx context.Context, serviceName string, port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	pm.listeners[port] = listener

	go pm.acceptLoop(ctx, serviceName, port, listener)
	return nil
}

func (pm *ProxyManager) acceptLoop(ctx context.Context, serviceName string, port int, listener net.Listener) {
	var rrIndex uint64
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		go pm.handleConn(ctx, conn, serviceName, &rrIndex)
	}
}

func (pm *ProxyManager) handleConn(ctx context.Context, clientConn net.Conn, serviceName string, rrIndex *uint64) {
	defer clientConn.Close()

	backends, err := pm.getReadyBackends(ctx, serviceName)
	if err != nil || len(backends) == 0 {
		return
	}

	// Round-robin selection
	idx := *rrIndex
	*rrIndex++
	target := backends[idx%uint64(len(backends))]

	targetAddr := fmt.Sprintf("%s:%d", target.Address, target.HostPort)
	backendConn, err := net.DialTimeout("tcp", targetAddr, 3*time.Second)
	if err != nil {
		log.Printf("Proxy error connecting to backend %s: %v", targetAddr, err)
		return
	}
	defer backendConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(backendConn, clientConn)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, backendConn)
	}()

	wg.Wait()
}

func (pm *ProxyManager) getReadyBackends(ctx context.Context, serviceName string) ([]Backend, error) {
	replicas, err := pm.store.ListReplicas(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := pm.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*types.Node)
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	var backends []Backend
	for _, rep := range replicas {
		if rep.ServiceName == serviceName && rep.Status == types.PodStatusRunning && rep.Health == types.HealthStatusHealthy && rep.Ready {
			if node, ok := nodeMap[rep.NodeName]; ok && node.Status == "Ready" {
				backends = append(backends, Backend{
					Address:  node.Address,
					HostPort: rep.HostPort,
				})
			}
		}
	}

	return backends, nil
}

func (pm *ProxyManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, l := range pm.listeners {
		_ = l.Close()
	}
}

