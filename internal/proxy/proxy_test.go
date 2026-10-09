package proxy

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

func TestProxyRoundRobinsReadyBackendsAndUnbindsWhenEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := storage.NewMapStorage()
	servicePort := testFreePort(t)
	if err := store.DeployService(ctx, &types.Service{
		Name: "web", Image: "whoami", Replicas: 2, ContainerPort: 80, ServicePort: servicePort,
	}); err != nil {
		t.Fatal(err)
	}

	type backendServer struct {
		listener net.Listener
		name     string
	}
	backends := make([]backendServer, 0, 2)
	replicas := make([]*types.Replica, 0, 2)
	for index, name := range []string{"one", "two"} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		backends = append(backends, backendServer{listener: listener, name: name})
		go func(server backendServer) {
			for {
				connection, acceptErr := server.listener.Accept()
				if acceptErr != nil {
					return
				}
				_, _ = io.WriteString(connection, server.name)
				_ = connection.Close()
			}
		}(backends[index])
		replicaID := "replica-" + name
		hostPort := listener.Addr().(*net.TCPAddr).Port
		replica := &types.Replica{
			ID: replicaID, ServiceName: "web", NodeName: "node-" + name,
			HostPort: hostPort, Status: types.PodStatusRunning,
			Health: types.HealthStatusUnknown, Ready: false,
		}
		replicas = append(replicas, replica)
		if err := store.UpsertNode(ctx, &types.Node{
			Name: replica.NodeName, Address: "127.0.0.1", Status: "Ready",
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.UpsertReplica(ctx, replica); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for _, backend := range backends {
			_ = backend.listener.Close()
		}
	}()

	manager := NewProxyManager(store, "127.0.0.1:9001")
	defer manager.Close()
	if err := manager.RestoreListeners(ctx); err != nil {
		t.Fatal(err)
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(servicePort))
	connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatal("proxy listener bound without ready endpoints")
	}
	for _, replica := range replicas {
		replica.Health = types.HealthStatusHealthy
		replica.Ready = true
		if err := store.UpsertReplica(ctx, replica); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(backendCacheTTL + 20*time.Millisecond)
	if err := manager.RestoreListeners(ctx); err != nil {
		t.Fatal(err)
	}
	responses := make(map[string]bool)
	for i := 0; i < 2; i++ {
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			t.Fatalf("dial proxy: %v", err)
		}
		_ = connection.SetReadDeadline(time.Now().Add(time.Second))
		body, err := io.ReadAll(connection)
		_ = connection.Close()
		if err != nil {
			t.Fatalf("read proxied response: %v", err)
		}
		responses[string(body)] = true
	}
	if !responses["one"] || !responses["two"] {
		t.Fatalf("round robin responses = %v, want both backends", responses)
	}

	for _, replica := range replicas {
		replica.Health = types.HealthStatusUnhealthy
		replica.Ready = false
		if err := store.UpsertReplica(ctx, replica); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(backendCacheTTL + 20*time.Millisecond)
	if err := manager.RestoreListeners(ctx); err != nil {
		t.Fatal(err)
	}
	connection, err = net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatal("proxy listener remained bound with no ready endpoints")
	}
	if !strings.Contains(err.Error(), "refused") {
		t.Fatalf("dial after unbind error = %v, want connection refused", err)
	}

	for _, replica := range replicas {
		replica.Health = types.HealthStatusHealthy
		replica.Ready = true
		if err := store.UpsertReplica(ctx, replica); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(backendCacheTTL + 20*time.Millisecond)
	if err := manager.RestoreListeners(ctx); err != nil {
		t.Fatal(err)
	}
	connection, err = net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatalf("proxy listener did not rebind when endpoints returned: %v", err)
	}
	_ = connection.Close()
}

func testFreePort(t *testing.T) int {
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
