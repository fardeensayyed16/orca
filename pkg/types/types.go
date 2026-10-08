package types

import "time"

type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"
)

type PullPolicy string

const (
	PullPolicyIfNotPresent PullPolicy = "IfNotPresent"
	PullPolicyAlways       PullPolicy = "Always"
	PullPolicyNever        PullPolicy = "Never"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "Healthy"
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	HealthStatusUnknown   HealthStatus = "Unknown"
)

type PodStatus string

const (
	PodStatusPending   PodStatus = "Pending"
	PodStatusStarting  PodStatus = "Starting"
	PodStatusRunning   PodStatus = "Running"
	PodStatusFailed    PodStatus = "Failed"
	PodStatusCompleted PodStatus = "Completed"
)

type TCPSocketAction struct {
	Port int `json:"port"`
}

type HTTPGetAction struct {
	Path string `json:"path"`
	Port int    `json:"port"`
}

type ExecAction struct {
	Command []string `json:"command"`
}

type Probe struct {
	HTTPGet             *HTTPGetAction   `json:"http_get,omitempty"`
	TCPSocket           *TCPSocketAction `json:"tcp_socket,omitempty"`
	Exec                *ExecAction      `json:"exec,omitempty"`
	InitialDelaySeconds int              `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int              `json:"period_seconds,omitempty"`
	TimeoutSeconds      int              `json:"timeout_seconds,omitempty"`
	FailureThreshold    int              `json:"failure_threshold,omitempty"`
	SuccessThreshold    int              `json:"success_threshold,omitempty"`
}

type AutoScaleConfig struct {
	Min                      int     `json:"min"`
	Max                      int     `json:"max"`
	TargetCPU                float64 `json:"target_cpu"`
	ScaleDownWindowSeconds   int     `json:"scale_down_window_seconds"`
	Tolerance                float64 `json:"tolerance"`
}

type Service struct {
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Replicas       int               `json:"replicas"`
	ContainerPort  int               `json:"container_port,omitempty"`
	ServicePort    int               `json:"service_port,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Command        []string          `json:"command,omitempty"`
	CPUMillis      int64             `json:"cpu_millis,omitempty"`
	MemoryBytes    int64             `json:"memory_bytes,omitempty"`
	RestartPolicy  RestartPolicy     `json:"restart_policy,omitempty"`
	PullPolicy     PullPolicy        `json:"pull_policy,omitempty"`
	LivenessProbe  *Probe            `json:"liveness_probe,omitempty"`
	ReadinessProbe *Probe            `json:"readiness_probe,omitempty"`
	Autoscale      *AutoScaleConfig  `json:"autoscale,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

type Replica struct {
	ID               string       `json:"id"`
	ServiceName      string       `json:"service_name"`
	NodeName         string       `json:"node_name,omitempty"`
	ContainerID      string       `json:"container_id,omitempty"`
	HostPort         int          `json:"host_port,omitempty"`
	Status           PodStatus    `json:"status"`
	Health           HealthStatus `json:"health"`
	Ready            bool         `json:"ready"`
	RestartCount     int          `json:"restart_count"`
	RescheduleCount  int          `json:"reschedule_count"`
	LastRescheduledAt *time.Time  `json:"last_rescheduled_at,omitempty"`
	ExitCode         int          `json:"exit_code"`
	Message          string       `json:"message,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	StartedAt        *time.Time   `json:"started_at,omitempty"`
}

type Node struct {
	Name                 string    `json:"name"`
	Address              string    `json:"address"`
	CapacityCPUMillis    int64     `json:"capacity_cpu_millis"`
	CapacityMemoryBytes  int64     `json:"capacity_memory_bytes"`
	Status               string    `json:"status"` // Ready | NotReady | Deleting
	LastSeen             time.Time `json:"last_seen"`
	NotReadySince        *time.Time`json:"not_ready_since,omitempty"`
	DockerOK             bool      `json:"docker_ok"`
}

type Metrics struct {
	ID          int64     `json:"id"`
	ReplicaID   string    `json:"replica_id"`
	Timestamp   time.Time `json:"timestamp"`
	CPUCores    float64   `json:"cpu_cores"`
	MemoryBytes int64     `json:"memory_bytes"`
}
