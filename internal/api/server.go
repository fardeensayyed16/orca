package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/fardeensayyed16/orca/internal/scheduler"
	"github.com/fardeensayyed16/orca/internal/serviceports"
	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

type Server struct {
	store            storage.Storage
	mux              *http.ServeMux
	token            string
	ports            *serviceports.Allocator
	portAllocatorErr error
	serviceMu        sync.Mutex
}

func NewServer(store storage.Storage, token string, allocators ...*serviceports.Allocator) *Server {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
		token: token,
	}
	if len(allocators) > 0 {
		s.ports = allocators[0]
	} else {
		s.ports, s.portAllocatorErr = serviceports.NewAllocator(store, serviceports.DefaultRange, "0.0.0.0:9001")
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// No-auth routes
	s.mux.HandleFunc("GET /api/v1/ping", s.handlePing)
	s.mux.HandleFunc("GET /api/v1/_test/list", s.handleTestList)
	s.mux.HandleFunc("GET /api/v1/_test/echo", s.handleTestEcho)

	// Token-protected node routes
	s.mux.HandleFunc("POST /api/v1/nodes/register", s.auth(s.handleRegisterNode))
	s.mux.HandleFunc("POST /api/v1/nodes/{name}/heartbeat", s.auth(s.handleHeartbeat))
	s.mux.HandleFunc("GET /api/v1/work/{nodeName}", s.auth(s.handleWork))
	s.mux.HandleFunc("GET /api/v1/nodes", s.auth(s.handleListNodes))
	s.mux.HandleFunc("GET /api/v1/nodes/{name}", s.auth(s.handleGetNode))
	s.mux.HandleFunc("DELETE /api/v1/nodes/{name}", s.auth(s.handleDeleteNode))

	// Token-protected service and pod routes
	s.mux.HandleFunc("POST /api/v1/services", s.auth(s.handleCreateService))
	s.mux.HandleFunc("GET /api/v1/services", s.auth(s.handleListServices))
	s.mux.HandleFunc("PATCH /api/v1/services/{name}/scale", s.auth(s.handleScaleService))
	s.mux.HandleFunc("DELETE /api/v1/services/{name}", s.auth(s.handleDeleteService))
	s.mux.HandleFunc("GET /api/v1/services/{name}/endpoints", s.auth(s.handleServiceEndpoints))
	s.mux.HandleFunc("GET /api/v1/pods", s.auth(s.handleListPods))

	// token/show is restricted to loopback callers instead of bearer-token auth.
	s.mux.HandleFunc("POST /api/v1/token/show", loopbackOnly(s.handleTokenShow))
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	want := []byte(s.token)
	return func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(hdr) <= len(prefix) || hdr[:len(prefix)] != prefix {
			jsonError(w, "missing or malformed Authorization header", http.StatusUnauthorized)
			return
		}
		got := []byte(hdr[len(prefix):])
		if subtle.ConstantTimeCompare(got, want) != 1 {
			jsonError(w, "invalid token", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Request/Response DTOs
type RegisterRequest struct {
	Name                string `json:"name"`
	Address             string `json:"address"`
	CapacityCPUMillis   int64  `json:"capacity_cpu_millis"`
	CapacityMemoryBytes int64  `json:"capacity_memory_bytes"`
	Runtime             string `json:"runtime"`
}

type HeartbeatRequest struct {
	Address             string             `json:"address"`
	ContainerIDs        []string           `json:"container_ids"`
	Replicas            []ReplicaHeartbeat `json:"replicas"`
	DockerOK            bool               `json:"docker_ok"`
	CapacityCPUMillis   int64              `json:"capacity_cpu_millis,omitempty"`
	CapacityMemoryBytes int64              `json:"capacity_memory_bytes,omitempty"`
}

type ReplicaHeartbeat struct {
	ID           string             `json:"id"`
	ContainerID  string             `json:"container_id"`
	HostPort     int                `json:"host_port"`
	Status       types.PodStatus    `json:"status"`
	Health       types.HealthStatus `json:"health"`
	Ready        bool               `json:"ready"`
	RestartCount int                `json:"restart_count"`
	ExitCode     int                `json:"exit_code"`
	Message      string             `json:"message,omitempty"`
}

type WorkReplica struct {
	ID                  string              `json:"id"`
	ServiceName         string              `json:"service_name"`
	Image               string              `json:"image"`
	CPUMillis           int64               `json:"cpu_millis"`
	MemoryBytes         int64               `json:"memory_bytes"`
	ContainerPort       int                 `json:"container_port"`
	DesiredRestartCount int                 `json:"desired_restart_count"`
	RestartPolicy       types.RestartPolicy `json:"restart_policy,omitempty"`
	PullPolicy          types.PullPolicy    `json:"pull_policy,omitempty"`
	Env                 map[string]string   `json:"env,omitempty"`
	Command             []string            `json:"command,omitempty"`
	Liveness            *types.Probe        `json:"liveness,omitempty"`
	Readiness           *types.Probe        `json:"readiness,omitempty"`
}

type CreateServiceRequest struct {
	Name          string              `json:"name"`
	Image         string              `json:"image"`
	Replicas      int                 `json:"replicas"`
	CPUMillis     int64               `json:"cpu_millis"`
	MemoryBytes   int64               `json:"memory_bytes"`
	ContainerPort int                 `json:"container_port"`
	RestartPolicy types.RestartPolicy `json:"restart_policy,omitempty"`
	PullPolicy    types.PullPolicy    `json:"pull_policy,omitempty"`
	Env           map[string]string   `json:"env,omitempty"`
	Command       []string            `json:"command,omitempty"`
	Liveness      *types.Probe        `json:"liveness,omitempty"`
	Readiness     *types.Probe        `json:"readiness,omitempty"`
}

type ReplicaSummary struct {
	Desired  int `json:"desired"`
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Starting int `json:"starting"`
	Running  int `json:"running"`
	Failed   int `json:"failed"`
}

type ServiceWithReplicaSummary struct {
	*types.Service
	ReplicaSummary ReplicaSummary `json:"replica_summary"`
}

type CreateServiceResponse struct {
	Service  *types.Service   `json:"service"`
	Replicas []*types.Replica `json:"replicas"`
}

func (s *Server) handleCreateService(w http.ResponseWriter, r *http.Request) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	var req CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Image == "" {
		jsonError(w, "name and image are required", http.StatusBadRequest)
		return
	}
	if req.Replicas == 0 {
		req.Replicas = 1
	}
	if req.Replicas < 0 || req.CPUMillis < 0 || req.MemoryBytes < 0 ||
		req.ContainerPort < 0 || req.ContainerPort > 65535 {
		jsonError(w, "replicas, resources, or container_port are invalid", http.StatusBadRequest)
		return
	}
	if req.RestartPolicy == "" {
		req.RestartPolicy = types.RestartPolicyAlways
	}
	if req.RestartPolicy != types.RestartPolicyAlways &&
		req.RestartPolicy != types.RestartPolicyOnFailure &&
		req.RestartPolicy != types.RestartPolicyNever {
		jsonError(w, "restart_policy must be Always, OnFailure, or Never", http.StatusBadRequest)
		return
	}
	if req.PullPolicy == "" {
		req.PullPolicy = types.PullPolicyIfNotPresent
	}
	if req.PullPolicy != types.PullPolicyIfNotPresent &&
		req.PullPolicy != types.PullPolicyAlways &&
		req.PullPolicy != types.PullPolicyNever {
		jsonError(w, "pull_policy must be IfNotPresent, Always, or Never", http.StatusBadRequest)
		return
	}
	if err := normalizeProbe(req.Liveness, "liveness", req.ContainerPort); err != nil {
		jsonError(w, "invalid liveness probe: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := normalizeProbe(req.Readiness, "readiness", req.ContainerPort); err != nil {
		jsonError(w, "invalid readiness probe: "+err.Error(), http.StatusBadRequest)
		return
	}

	existing, err := s.store.GetService(r.Context(), req.Name)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		jsonError(w, "service already exists", http.StatusConflict)
		return
	}

	if req.CPUMillis == 0 {
		req.CPUMillis = 1000
	}
	if req.MemoryBytes == 0 {
		req.MemoryBytes = 536870912
	}
	servicePort := 0
	if req.ContainerPort > 0 {
		if s.portAllocatorErr != nil {
			jsonError(w, "service port allocator unavailable: "+s.portAllocatorErr.Error(), http.StatusInternalServerError)
			return
		}
		if s.ports == nil {
			jsonError(w, "service port allocator is not configured", http.StatusInternalServerError)
			return
		}
		servicePort, err = s.ports.Allocate(r.Context(), req.Name)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	service := &types.Service{
		Name:          req.Name,
		Image:         req.Image,
		Replicas:      req.Replicas,
		CPUMillis:     req.CPUMillis,
		MemoryBytes:   req.MemoryBytes,
		ContainerPort: req.ContainerPort,
		ServicePort:   servicePort,
		RestartPolicy: req.RestartPolicy,
		PullPolicy:    req.PullPolicy,
		Env:           req.Env,
		Command:       req.Command,
		Liveness:      req.Liveness,
		Readiness:     req.Readiness,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := s.store.DeployService(r.Context(), service); err != nil {
		if servicePort > 0 {
			s.ports.Release(service.Name, servicePort)
		}
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	replicas := make([]*types.Replica, 0, req.Replicas)
	for i := 0; i < req.Replicas; i++ {
		id, err := newReplicaID()
		if err != nil {
			jsonError(w, "failed to create replica ID: "+err.Error(), http.StatusInternalServerError)
			return
		}
		replica := &types.Replica{
			ID:          id,
			ServiceName: service.Name,
			Status:      types.PodStatusPending,
			Health:      types.HealthStatusUnknown,
			CreatedAt:   time.Now().UTC(),
		}
		if err := s.store.UpsertReplica(r.Context(), replica); err != nil {
			jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.store.WithTx(r.Context(), func(tx storage.Tx) error {
			return scheduler.ScheduleReplica(r.Context(), tx, service, replica)
		}); err != nil && !errors.Is(err, scheduler.ErrNoSuitableNode) {
			jsonError(w, "scheduling error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		replicas = append(replicas, replica)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CreateServiceResponse{Service: service, Replicas: replicas})
}

func (s *Server) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	name := r.PathValue("name")
	service, err := s.store.GetService(r.Context(), name)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		jsonError(w, "service not found", http.StatusNotFound)
		return
	}
	if err := s.store.DeleteService(r.Context(), name); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if s.ports != nil && service.ServicePort > 0 {
		s.ports.Release(name, service.ServicePort)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScaleService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Replicas <= 0 {
		jsonError(w, "replicas must be greater than zero", http.StatusBadRequest)
		return
	}

	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	service, err := s.store.GetService(r.Context(), r.PathValue("name"))
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		jsonError(w, "service not found", http.StatusNotFound)
		return
	}
	service.Replicas = req.Replicas
	service.UpdatedAt = time.Now().UTC()
	if err := s.store.DeployService(r.Context(), service); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(service)
}

func normalizeProbe(probe *types.Probe, kind string, containerPort int) error {
	if probe == nil {
		return nil
	}
	if probe.Kind == "" {
		probe.Kind = kind
	}
	if probe.Kind != kind {
		return fmt.Errorf("kind must be %q", kind)
	}
	switch probe.Type {
	case "http":
		if containerPort == 0 {
			return errors.New("HTTP probes require a published container_port")
		}
		if probe.Port != 0 && probe.Port != containerPort {
			return fmt.Errorf("HTTP probe port must match container_port %d", containerPort)
		}
		probe.Port = containerPort
		if probe.Path == "" {
			probe.Path = "/"
		}
		if !strings.HasPrefix(probe.Path, "/") {
			return errors.New("HTTP probe path must begin with '/'")
		}
	case "tcp":
		if containerPort == 0 {
			return errors.New("TCP probes require a published container_port")
		}
		if probe.Port != 0 && probe.Port != containerPort {
			return fmt.Errorf("TCP probe port must match container_port %d", containerPort)
		}
		probe.Port = containerPort
	case "exec":
		if len(probe.Command) == 0 || strings.TrimSpace(probe.Command[0]) == "" {
			return errors.New("exec probes require a command")
		}
	default:
		return errors.New("type must be http, tcp, or exec")
	}
	if probe.InitialDelaySeconds < 0 {
		return errors.New("initial_delay_seconds cannot be negative")
	}
	if probe.PeriodSeconds == 0 {
		probe.PeriodSeconds = 10
	}
	if probe.PeriodSeconds < 1 {
		return errors.New("period_seconds must be positive")
	}
	if probe.TimeoutSeconds == 0 {
		probe.TimeoutSeconds = 1
	}
	if probe.TimeoutSeconds < 1 {
		return errors.New("timeout_seconds must be positive")
	}
	if probe.FailureThreshold == 0 {
		probe.FailureThreshold = 3
	}
	if probe.FailureThreshold < 1 {
		return errors.New("failure_threshold must be positive")
	}
	if probe.SuccessThreshold == 0 {
		probe.SuccessThreshold = 1
	}
	if probe.SuccessThreshold < 1 {
		return errors.New("success_threshold must be positive")
	}
	return nil
}

type ServiceEndpoint struct {
	ReplicaID   string `json:"replica_id"`
	NodeAddress string `json:"node_address"`
	HostPort    int    `json:"host_port"`
}

func (s *Server) handleServiceEndpoints(w http.ResponseWriter, r *http.Request) {
	serviceName := r.PathValue("name")
	service, err := s.store.GetService(r.Context(), serviceName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if service == nil {
		jsonError(w, "service not found", http.StatusNotFound)
		return
	}
	replicas, err := s.store.ListReplicasByService(r.Context(), serviceName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	nodes, err := s.store.ListNodes(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	nodesByName := make(map[string]*types.Node, len(nodes))
	for _, node := range nodes {
		nodesByName[node.Name] = node
	}
	endpoints := make([]ServiceEndpoint, 0)
	for _, replica := range replicas {
		if replica.Status != types.PodStatusRunning ||
			replica.Health != types.HealthStatusHealthy ||
			!replica.Ready || replica.HostPort <= 0 {
			continue
		}
		node := nodesByName[replica.NodeName]
		if node == nil || node.Status != "Ready" {
			continue
		}
		endpoints = append(endpoints, ServiceEndpoint{
			ReplicaID: replica.ID, NodeAddress: node.Address, HostPort: replica.HostPort,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(endpoints)
}

func newReplicaID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.store.ListServices(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	replicas, err := s.store.ListReplicas(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	summaries := make(map[string]*ReplicaSummary, len(services))
	for _, service := range services {
		summaries[service.Name] = &ReplicaSummary{Desired: service.Replicas}
	}
	for _, replica := range replicas {
		summary := summaries[replica.ServiceName]
		if summary == nil {
			continue
		}
		summary.Total++
		switch replica.Status {
		case types.PodStatusPending:
			summary.Pending++
		case types.PodStatusStarting:
			summary.Starting++
		case types.PodStatusRunning:
			summary.Running++
		case types.PodStatusFailed:
			summary.Failed++
		}
	}
	result := make([]ServiceWithReplicaSummary, 0, len(services))
	for _, service := range services {
		result = append(result, ServiceWithReplicaSummary{
			Service:        service,
			ReplicaSummary: *summaries[service.Name],
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	replicas, err := s.store.ListReplicas(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if replicas == nil {
		replicas = []*types.Replica{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(replicas)
}

func (s *Server) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	// Address resolution: explicit address or RemoteAddr fallback
	addrStr := req.Address
	if addrStr == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			addrStr = r.RemoteAddr
		} else {
			addrStr = host
		}
	}

	// Validate IP address format
	if _, err := netip.ParseAddr(addrStr); err != nil {
		jsonError(w, "invalid address format: "+addrStr, http.StatusBadRequest)
		return
	}

	existing, err := s.store.GetNode(r.Context(), req.Name)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if existing != nil {
		// Rule E: Same name AND different address -> 409 Conflict
		if existing.Address != addrStr {
			jsonError(w, "node name already registered with a different address", http.StatusConflict)
			return
		}

		// Same name AND same address -> 200 OK (upsert/update capacities)
		if req.CapacityCPUMillis > 0 {
			existing.CapacityCPUMillis = req.CapacityCPUMillis
		}
		if req.CapacityMemoryBytes > 0 {
			existing.CapacityMemoryBytes = req.CapacityMemoryBytes
		}
		existing.LastSeen = time.Now().UTC()
		if existing.Status != "Deleting" {
			existing.Status = "Ready"
			existing.NotReadySince = nil
		}
		existing.DockerOK = true

		if err := s.store.UpsertNode(r.Context(), existing); err != nil {
			jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	// Unknown name -> 201 Created
	newNode := &types.Node{
		Name:                req.Name,
		Address:             addrStr,
		CapacityCPUMillis:   req.CapacityCPUMillis,
		CapacityMemoryBytes: req.CapacityMemoryBytes,
		Status:              "Ready",
		LastSeen:            time.Now().UTC(),
		DockerOK:            true,
	}

	if err := s.store.UpsertNode(r.Context(), newNode); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(newNode)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("name")
	if nodeName == "" {
		jsonError(w, "node name required", http.StatusBadRequest)
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	existing, err := s.store.GetNode(r.Context(), nodeName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if existing == nil {
		// Unknown node -> 404
		jsonError(w, "node not found, re-registration required", http.StatusNotFound)
		return
	}

	// Check address mismatch -> 409
	if req.Address != "" && req.Address != existing.Address {
		jsonError(w, "address mismatch during heartbeat", http.StatusConflict)
		return
	}

	// Update node in DB including capacity if zero or updated and docker_ok status
	existing.LastSeen = time.Now().UTC()
	if existing.Status != "Deleting" {
		existing.Status = "Ready"
		existing.NotReadySince = nil
	}
	existing.DockerOK = req.DockerOK
	if existing.CapacityCPUMillis == 0 && req.CapacityCPUMillis > 0 {
		existing.CapacityCPUMillis = req.CapacityCPUMillis
	}
	if existing.CapacityMemoryBytes == 0 && req.CapacityMemoryBytes > 0 {
		existing.CapacityMemoryBytes = req.CapacityMemoryBytes
	}

	if err := s.store.UpsertNode(r.Context(), existing); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for _, reported := range req.Replicas {
		assigned, err := s.store.GetReplica(r.Context(), reported.ID)
		if err != nil {
			jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if assigned == nil || assigned.NodeName != nodeName {
			continue
		}
		replica := &types.Replica{
			ID:           reported.ID,
			ContainerID:  reported.ContainerID,
			HostPort:     reported.HostPort,
			Status:       reported.Status,
			Health:       reported.Health,
			Ready:        reported.Ready,
			RestartCount: reported.RestartCount,
			ExitCode:     reported.ExitCode,
			Message:      reported.Message,
		}
		if err := s.store.UpdateReplicaHeartbeat(r.Context(), nodeName, replica); err != nil {
			jsonError(w, "replica heartbeat error: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleWork(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("nodeName")
	node, err := s.store.GetNode(r.Context(), nodeName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	desired := make([]WorkReplica, 0)
	if node == nil || node.Status == "Deleting" || node.Status == "Draining" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(desired)
		return
	}
	replicas, err := s.store.ListReplicas(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	services, err := s.store.ListServices(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	servicesByName := make(map[string]*types.Service, len(services))
	for _, service := range services {
		servicesByName[service.Name] = service
	}
	for _, replica := range replicas {
		if replica.NodeName != nodeName || replica.Status == types.PodStatusPending {
			continue
		}
		service := servicesByName[replica.ServiceName]
		if service == nil {
			jsonError(w, "service for replica "+replica.ID+" not found", http.StatusInternalServerError)
			return
		}
		desired = append(desired, WorkReplica{
			ID:                  replica.ID,
			ServiceName:         service.Name,
			Image:               service.Image,
			CPUMillis:           service.CPUMillis,
			MemoryBytes:         service.MemoryBytes,
			ContainerPort:       service.ContainerPort,
			DesiredRestartCount: replica.RestartCount,
			RestartPolicy:       service.RestartPolicy,
			PullPolicy:          service.PullPolicy,
			Env:                 service.Env,
			Command:             service.Command,
			Liveness:            service.Liveness,
			Readiness:           service.Readiness,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(desired)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("name")
	node, err := s.store.GetNode(r.Context(), nodeName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if node == nil {
		jsonError(w, "node not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(node)
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeName := r.PathValue("name")
	if nodeName == "" {
		jsonError(w, "node name required", http.StatusBadRequest)
		return
	}
	node, err := s.store.GetNode(r.Context(), nodeName)
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if node == nil {
		jsonError(w, "node not found", http.StatusNotFound)
		return
	}
	replicas, err := s.store.ListReplicas(r.Context())
	if err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	hasReplicas := false
	for _, replica := range replicas {
		if replica.NodeName == nodeName {
			hasReplicas = true
			break
		}
	}
	if !hasReplicas {
		if err := s.store.DeleteNode(r.Context(), nodeName); err != nil {
			jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	node.Status = "Deleting"
	if node.NotReadySince == nil {
		now := time.Now().UTC()
		node.NotReadySince = &now
	}
	if err := s.store.UpsertNode(r.Context(), node); err != nil {
		jsonError(w, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleTestList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "smoke-test-ok"})
}

func (s *Server) handleTestEcho(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"echo": r.URL.Query().Get("msg")})
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes(r.Context())
	if err != nil {
		jsonError(w, "internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if nodes == nil {
		nodes = []*types.Node{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

func (s *Server) handleTokenShow(w http.ResponseWriter, r *http.Request) {
	tok, err := s.store.GetToken(r.Context())
	if err != nil {
		jsonError(w, "internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": tok})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
