package serviceports

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestAllocatorRestoresPortsAndReusesReleasedPort(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	first := freePort(t)
	if first == 65535 {
		t.Fatal("cannot find adjacent test ports")
	}
	second := first + 1
	if err := store.DeployService(ctx, &types.Service{Name: "persisted", Image: "nginx", ServicePort: first}); err != nil {
		t.Fatal(err)
	}
	allocator, err := NewAllocator(store, portRange(first, second), "127.0.0.1:9001")
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	port, err := allocator.Allocate(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if port != second {
		t.Fatalf("allocated port = %d, want %d", port, second)
	}
	allocator.Release("web", port)
	reused, err := allocator.Allocate(ctx, "another")
	if err != nil {
		t.Fatal(err)
	}
	if reused != port {
		t.Fatalf("reused port = %d, want %d", reused, port)
	}
}

func TestAllocatorReportsExhaustionAndExternalBindFailure(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	exhaustionPort := freePort(t)
	allocator, err := NewAllocator(store, portRange(exhaustionPort, exhaustionPort), "127.0.0.1:9001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := allocator.Allocate(ctx, "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := allocator.Allocate(ctx, "second"); err == nil || err.Error() != "no free host ports available" {
		t.Fatalf("exhaustion error = %v, want exact no-free-ports message", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	externalPort := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()
	boundAllocator, err := NewAllocator(store, portRange(externalPort, externalPort), "127.0.0.1:9001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := boundAllocator.Allocate(ctx, "blocked"); err == nil || !strings.Contains(err.Error(), "cannot be bound") {
		t.Fatalf("external bind error = %v, want a clear bind failure", err)
	}
}

func TestAllocatorKeepsPersistedPortsOutsideConfiguredRangeReserved(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMapStorage()
	configuredPort := freePort(t)
	persistedPort := configuredPort - 1
	if persistedPort < 1 {
		persistedPort = configuredPort + 1
	}
	if err := store.DeployService(ctx, &types.Service{Name: "old", Image: "nginx", ServicePort: persistedPort}); err != nil {
		t.Fatal(err)
	}
	allocator, err := NewAllocator(store, portRange(configuredPort, configuredPort), "127.0.0.1:9001")
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	port, err := allocator.Allocate(ctx, "new")
	if err != nil {
		t.Fatal(err)
	}
	if port != configuredPort {
		t.Fatalf("allocated port = %d, want %d", port, configuredPort)
	}
}

func TestAllocatorRestoresSQLiteAllocations(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	persistedPort := freePort(t)
	if err := store.DeployService(ctx, &types.Service{
		Name: "persisted", Image: "nginx", ServicePort: persistedPort,
	}); err != nil {
		t.Fatal(err)
	}
	if persistedPort == 65535 {
		t.Fatal("cannot find adjacent test ports")
	}
	allocator, err := NewAllocator(
		store,
		portRange(persistedPort, persistedPort+1),
		"127.0.0.1:9001",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	port, err := allocator.Allocate(ctx, "new")
	if err != nil {
		t.Fatal(err)
	}
	if port != persistedPort+1 {
		t.Fatalf("allocated port = %d, want %d", port, persistedPort+1)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func portRange(first, last int) string {
	return strconv.Itoa(first) + "-" + strconv.Itoa(last)
}
