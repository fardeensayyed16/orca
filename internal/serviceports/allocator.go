package serviceports

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/fardeensayyed16/orca/internal/storage"
)

const DefaultRange = "30000-32767"

type Allocator struct {
	store    storage.Storage
	first    int
	last     int
	bindIP   string
	mu       sync.Mutex
	used     map[int]string
	restored bool
}

func NewAllocator(store storage.Storage, portRange, bindAddress string) (*Allocator, error) {
	if portRange == "" {
		portRange = DefaultRange
	}
	first, last, err := parseRange(portRange)
	if err != nil {
		return nil, err
	}
	bindIP := "0.0.0.0"
	if host, _, err := net.SplitHostPort(bindAddress); err == nil && host != "" {
		bindIP = host
	}
	return &Allocator{store: store, first: first, last: last, bindIP: bindIP, used: make(map[int]string)}, nil
}

func parseRange(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid service_port_range %q: expected start-end", value)
	}
	first, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid service_port_range start: %w", err)
	}
	last, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid service_port_range end: %w", err)
	}
	if first < 1 || last > 65535 || first > last {
		return 0, 0, fmt.Errorf("invalid service_port_range %q: ports must be within 1-65535 and start <= end", value)
	}
	return first, last, nil
}

func (a *Allocator) Restore(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.restoreLocked(ctx)
}

func (a *Allocator) restoreLocked(ctx context.Context) error {
	if a.restored {
		return nil
	}
	services, err := a.store.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("restore service port allocations: %w", err)
	}
	for _, service := range services {
		if service.ServicePort <= 0 {
			continue
		}
		if owner, exists := a.used[service.ServicePort]; exists && owner != service.Name {
			return fmt.Errorf("service port %d is allocated to both %q and %q", service.ServicePort, owner, service.Name)
		}
		a.used[service.ServicePort] = service.Name
	}
	a.restored = true
	return nil
}

func (a *Allocator) Allocate(ctx context.Context, serviceName string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.restoreLocked(ctx); err != nil {
		return 0, err
	}
	for port := a.first; port <= a.last; port++ {
		if _, used := a.used[port]; used {
			continue
		}
		address := net.JoinHostPort(a.bindIP, strconv.Itoa(port))
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return 0, fmt.Errorf("service port %d cannot be bound: %w", port, err)
		}
		if err := listener.Close(); err != nil {
			return 0, fmt.Errorf("release service port %d after availability check: %w", port, err)
		}
		a.used[port] = serviceName
		return port, nil
	}
	return 0, fmt.Errorf("no free host ports available")
}

func (a *Allocator) Release(serviceName string, port int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.used[port] == serviceName {
		delete(a.used, port)
	}
}
