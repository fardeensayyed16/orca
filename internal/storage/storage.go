package storage

import (
	"context"
	"time"

	"github.com/fardeensayyed16/orca/pkg/types"
)

type Tx interface {
	Commit() error
	Rollback() error
	ListServices(ctx context.Context) ([]*types.Service, error)
	ListNodes(ctx context.Context) ([]*types.Node, error)
	ListReplicas(ctx context.Context) ([]*types.Replica, error)
	UpdateReplicaNode(ctx context.Context, id, nodeName string) error
}

type Storage interface {
	// Service methods
	DeployService(ctx context.Context, svc *types.Service) error
	ListServices(ctx context.Context) ([]*types.Service, error)
	GetService(ctx context.Context, name string) (*types.Service, error)
	DeleteService(ctx context.Context, name string) error

	// Replica methods
	ListReplicas(ctx context.Context) ([]*types.Replica, error)
	ListReplicasByService(ctx context.Context, serviceName string) ([]*types.Replica, error)
	GetReplica(ctx context.Context, id string) (*types.Replica, error)
	UpsertReplica(ctx context.Context, replica *types.Replica) error
	UpdateReplicaNode(ctx context.Context, id, nodeName string) error
	DeleteReplica(ctx context.Context, id string) error

	// Node methods
	ListNodes(ctx context.Context) ([]*types.Node, error)
	GetNode(ctx context.Context, name string) (*types.Node, error)
	UpsertNode(ctx context.Context, node *types.Node) error
	MarkNodeReady(ctx context.Context, name string) error
	MarkNodeNotReady(ctx context.Context, name string) error
	RecordHeartbeat(ctx context.Context, name string, dockerOK bool) error

	// Metric methods
	InsertMetric(ctx context.Context, m *types.Metrics) error
	PruneOldMetrics(ctx context.Context, olderThan time.Duration) error

	// Config / Token methods
	GetToken(ctx context.Context) (string, error)
	SetToken(ctx context.Context, token string) error

	// Transaction support
	WithTx(ctx context.Context, fn func(tx Tx) error) error
	Close() error
}
