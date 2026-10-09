package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

const (
	backendCacheTTL = time.Second
	dialTimeout     = 2 * time.Second
	startupGrace    = 3 * time.Second
	syncInterval    = time.Second
)

type Backend struct {
	Address  string
	HostPort int
}

type listenerState struct {
	serviceName string
	listener    net.Listener
}

type backendCacheEntry struct {
	backends []Backend
	expires  time.Time
}

type ProxyManager struct {
	mu        sync.Mutex
	listeners map[int]listenerState
	bindLogs  map[int]time.Time
	cacheMu   sync.Mutex
	cache     map[string]backendCacheEntry
	rrMu      sync.Mutex
	rr        map[string]*atomic.Uint64
	store     storage.Storage
	bindAddr  string
}

func NewProxyManager(store storage.Storage, bindAddr string) *ProxyManager {
	return &ProxyManager{
		listeners: make(map[int]listenerState),
		bindLogs:  make(map[int]time.Time),
		cache:     make(map[string]backendCacheEntry),
		rr:        make(map[string]*atomic.Uint64),
		store:     store,
		bindAddr:  bindAddr,
	}
}

func (pm *ProxyManager) Start(ctx context.Context) {
	timer := time.NewTimer(startupGrace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	if err := pm.syncListeners(ctx); err != nil {
		log.Printf("Proxy listener reconciliation failed: %v", err)
	}
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := pm.syncListeners(ctx); err != nil {
				log.Printf("Proxy listener reconciliation failed: %v", err)
			}
		}
	}
}

// RestoreListeners performs one listener reconciliation immediately.
func (pm *ProxyManager) RestoreListeners(ctx context.Context) error {
	return pm.syncListeners(ctx)
}

func (pm *ProxyManager) syncListeners(ctx context.Context) error {
	services, err := pm.store.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("list services for proxy reconciliation: %w", err)
	}
	wanted := make(map[int]string)
	for _, service := range services {
		if service.ServicePort <= 0 {
			continue
		}
		backends, err := pm.getReadyBackends(ctx, service.Name)
		if err != nil {
			return fmt.Errorf("find ready backends for service %q: %w", service.Name, err)
		}
		if len(backends) > 0 {
			wanted[service.ServicePort] = service.Name
		}
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	for port, current := range pm.listeners {
		if serviceName, ok := wanted[port]; !ok || serviceName != current.serviceName {
			if err := current.listener.Close(); err != nil {
				log.Printf("Close proxy listener for service %s on port %d: %v", current.serviceName, port, err)
			}
			delete(pm.listeners, port)
		}
	}
	for port := range pm.bindLogs {
		if _, stillWanted := wanted[port]; !stillWanted {
			delete(pm.bindLogs, port)
		}
	}
	for port, serviceName := range wanted {
		if _, exists := pm.listeners[port]; exists {
			continue
		}
		if err := pm.startListenerLocked(ctx, serviceName, port); err != nil {
			now := time.Now()
			if lastLog, logged := pm.bindLogs[port]; !logged || now.Sub(lastLog) >= 10*time.Second {
				log.Printf("Failed to bind proxy listener on port %d for service %s: %v", port, serviceName, err)
				pm.bindLogs[port] = now
			}
		} else {
			delete(pm.bindLogs, port)
		}
	}
	return nil
}

func (pm *ProxyManager) startListenerLocked(ctx context.Context, serviceName string, port int) error {
	host := "0.0.0.0"
	if parsedHost, _, err := net.SplitHostPort(pm.bindAddr); err == nil && parsedHost != "" {
		host = parsedHost
	} else if pm.bindAddr != "" && !strings.Contains(pm.bindAddr, ":") {
		host = pm.bindAddr
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	pm.listeners[port] = listenerState{serviceName: serviceName, listener: listener}
	go pm.acceptLoop(ctx, serviceName, port, listener)
	return nil
}

func (pm *ProxyManager) acceptLoop(ctx context.Context, serviceName string, port int, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				pm.mu.Lock()
				current, exists := pm.listeners[port]
				if exists && current.listener == listener {
					delete(pm.listeners, port)
				}
				pm.mu.Unlock()
				return
			}
		}
		go pm.handleConn(ctx, conn, serviceName)
	}
}

func (pm *ProxyManager) handleConn(ctx context.Context, clientConn net.Conn, serviceName string) {
	defer clientConn.Close()
	backends, err := pm.getReadyBackends(ctx, serviceName)
	if err != nil {
		log.Printf("Proxy endpoint lookup for service %s failed: %v", serviceName, err)
		return
	}
	if len(backends) == 0 {
		return
	}
	rr := pm.roundRobin(serviceName)
	start := rr.Add(1) - 1
	var backendConn net.Conn
	for offset := uint64(0); offset < uint64(len(backends)); offset++ {
		target := backends[(start+offset)%uint64(len(backends))]
		targetAddr := net.JoinHostPort(target.Address, strconv.Itoa(target.HostPort))
		connection, dialErr := net.DialTimeout("tcp", targetAddr, dialTimeout)
		if dialErr != nil {
			log.Printf("Proxy error connecting to backend %s: %v", targetAddr, dialErr)
			continue
		}
		backendConn = connection
		break
	}
	if backendConn == nil {
		return
	}
	defer backendConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(backendConn, clientConn)
		if tcpConn, ok := backendConn.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, backendConn)
		if tcpConn, ok := clientConn.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
	}()
	wg.Wait()
}

func (pm *ProxyManager) roundRobin(serviceName string) *atomic.Uint64 {
	pm.rrMu.Lock()
	defer pm.rrMu.Unlock()
	counter := pm.rr[serviceName]
	if counter == nil {
		counter = &atomic.Uint64{}
		pm.rr[serviceName] = counter
	}
	return counter
}

func (pm *ProxyManager) Unbind(port int) error {
	pm.mu.Lock()
	state, exists := pm.listeners[port]
	if exists {
		delete(pm.listeners, port)
	}
	delete(pm.bindLogs, port)
	pm.mu.Unlock()
	if !exists {
		return nil
	}
	pm.cacheMu.Lock()
	delete(pm.cache, state.serviceName)
	pm.cacheMu.Unlock()
	if err := state.listener.Close(); err != nil {
		return fmt.Errorf("close proxy listener on port %d: %w", port, err)
	}
	return nil
}

func (pm *ProxyManager) getReadyBackends(ctx context.Context, serviceName string) ([]Backend, error) {
	now := time.Now()
	pm.cacheMu.Lock()
	if entry, ok := pm.cache[serviceName]; ok && now.Before(entry.expires) {
		backends := append([]Backend(nil), entry.backends...)
		pm.cacheMu.Unlock()
		return backends, nil
	}
	pm.cacheMu.Unlock()

	replicas, err := pm.store.ListReplicasByService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	nodes, err := pm.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodeMap := make(map[string]*types.Node, len(nodes))
	for _, node := range nodes {
		nodeMap[node.Name] = node
	}
	backends := make([]Backend, 0, len(replicas))
	for _, replica := range replicas {
		if replica.Status != types.PodStatusRunning ||
			replica.Health != types.HealthStatusHealthy ||
			!replica.Ready || replica.HostPort <= 0 {
			continue
		}
		node := nodeMap[replica.NodeName]
		if node == nil || node.Status != "Ready" || node.Address == "" {
			continue
		}
		backends = append(backends, Backend{Address: node.Address, HostPort: replica.HostPort})
	}

	pm.cacheMu.Lock()
	pm.cache[serviceName] = backendCacheEntry{
		backends: append([]Backend(nil), backends...),
		expires:  time.Now().Add(backendCacheTTL),
	}
	pm.cacheMu.Unlock()
	return backends, nil
}

func (pm *ProxyManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for port, state := range pm.listeners {
		if err := state.listener.Close(); err != nil {
			log.Printf("Close proxy listener on port %d: %v", port, err)
		}
		delete(pm.listeners, port)
	}
}
