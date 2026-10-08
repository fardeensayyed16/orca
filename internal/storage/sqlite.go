package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fardeensayyed16/orca/pkg/types"
	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
}

type sqliteTx struct {
	tx *sql.Tx
}

func (s *sqliteTx) Commit() error {
	return s.tx.Commit()
}

func (s *sqliteTx) Rollback() error {
	return s.tx.Rollback()
}

func NewSQLiteStorage(dataDir string) (*SQLiteStorage, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dataDir = filepath.Join(home, ".orca")
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "orca.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// PRAGMAs & connection pooling
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &SQLiteStorage{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS services (
		name TEXT PRIMARY KEY,
		image TEXT NOT NULL,
		replicas INTEGER NOT NULL,
		container_port INTEGER,
		service_port INTEGER,
		env_json TEXT,
		command_json TEXT,
		cpu_millis INTEGER,
		memory_bytes INTEGER,
		restart_policy TEXT,
		pull_policy TEXT,
		liveness_probe_json TEXT,
		readiness_probe_json TEXT,
		autoscale_json TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS replicas (
		id TEXT PRIMARY KEY,
		service_name TEXT NOT NULL,
		node_name TEXT,
		container_id TEXT,
		host_port INTEGER,
		status TEXT NOT NULL,
		health TEXT NOT NULL,
		ready INTEGER NOT NULL DEFAULT 0,
		restart_count INTEGER NOT NULL DEFAULT 0,
		reschedule_count INTEGER NOT NULL DEFAULT 0,
		last_rescheduled_at TEXT,
		exit_code INTEGER NOT NULL DEFAULT 0,
		message TEXT,
		created_at TEXT NOT NULL,
		started_at TEXT,
		FOREIGN KEY(service_name) REFERENCES services(name) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS nodes (
		name TEXT PRIMARY KEY,
		address TEXT NOT NULL,
		capacity_cpu_millis INTEGER NOT NULL,
		capacity_memory_bytes INTEGER NOT NULL,
		status TEXT NOT NULL,
		last_seen TEXT NOT NULL,
		not_ready_since TEXT,
		docker_ok INTEGER NOT NULL DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replica_id TEXT NOT NULL,
		ts TEXT NOT NULL,
		cpu_cores REAL NOT NULL,
		memory_bytes INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_replica_ts ON metrics(replica_id, ts);

	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) WithTx(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stx := &sqliteTx{tx: tx}
	if err := fn(stx); err != nil {
		_ = stx.Rollback()
		return err
	}
	return stx.Commit()
}

// Services

func (s *SQLiteStorage) DeployService(ctx context.Context, svc *types.Service) error {
	envJSON, _ := json.Marshal(svc.Env)
	cmdJSON, _ := json.Marshal(svc.Command)
	liveJSON, _ := json.Marshal(svc.LivenessProbe)
	readyJSON, _ := json.Marshal(svc.ReadinessProbe)
	autoJSON, _ := json.Marshal(svc.Autoscale)

	now := time.Now().UTC().Format(time.RFC3339)
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = time.Now().UTC()
	}
	svc.UpdatedAt = time.Now().UTC()

	query := `
	INSERT INTO services (
		name, image, replicas, container_port, service_port, env_json, command_json,
		cpu_millis, memory_bytes, restart_policy, pull_policy, liveness_probe_json,
		readiness_probe_json, autoscale_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		image=excluded.image,
		replicas=excluded.replicas,
		container_port=excluded.container_port,
		service_port=excluded.service_port,
		env_json=excluded.env_json,
		command_json=excluded.command_json,
		cpu_millis=excluded.cpu_millis,
		memory_bytes=excluded.memory_bytes,
		restart_policy=excluded.restart_policy,
		pull_policy=excluded.pull_policy,
		liveness_probe_json=excluded.liveness_probe_json,
		readiness_probe_json=excluded.readiness_probe_json,
		autoscale_json=excluded.autoscale_json,
		updated_at=excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query,
		svc.Name, svc.Image, svc.Replicas, svc.ContainerPort, svc.ServicePort,
		string(envJSON), string(cmdJSON), svc.CPUMillis, svc.MemoryBytes,
		string(svc.RestartPolicy), string(svc.PullPolicy), string(liveJSON),
		string(readyJSON), string(autoJSON), svc.CreatedAt.Format(time.RFC3339), now,
	)
	return err
}

func (s *SQLiteStorage) ListServices(ctx context.Context) ([]*types.Service, error) {
	query := `SELECT name, image, replicas, container_port, service_port, env_json, command_json,
		cpu_millis, memory_bytes, restart_policy, pull_policy, liveness_probe_json,
		readiness_probe_json, autoscale_json, created_at, updated_at FROM services`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*types.Service
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, svc)
	}
	return list, nil
}

func (s *SQLiteStorage) GetService(ctx context.Context, name string) (*types.Service, error) {
	query := `SELECT name, image, replicas, container_port, service_port, env_json, command_json,
		cpu_millis, memory_bytes, restart_policy, pull_policy, liveness_probe_json,
		readiness_probe_json, autoscale_json, created_at, updated_at FROM services WHERE name = ?`
	row := s.db.QueryRowContext(ctx, query, name)
	svc, err := scanService(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return svc, err
}

func (s *SQLiteStorage) DeleteService(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM services WHERE name = ?", name)
	return err
}

func scanService(scanner interface {
	Scan(dest ...any) error
}) (*types.Service, error) {
	var svc types.Service
	var envStr, cmdStr, liveStr, readyStr, autoStr, createdStr, updatedStr string
	var restartPol, pullPol string

	err := scanner.Scan(
		&svc.Name, &svc.Image, &svc.Replicas, &svc.ContainerPort, &svc.ServicePort,
		&envStr, &cmdStr, &svc.CPUMillis, &svc.MemoryBytes,
		&restartPol, &pullPol, &liveStr, &readyStr, &autoStr,
		&createdStr, &updatedStr,
	)
	if err != nil {
		return nil, err
	}

	svc.RestartPolicy = types.RestartPolicy(restartPol)
	svc.PullPolicy = types.PullPolicy(pullPol)
	_ = json.Unmarshal([]byte(envStr), &svc.Env)
	_ = json.Unmarshal([]byte(cmdStr), &svc.Command)
	if liveStr != "" && liveStr != "null" {
		svc.LivenessProbe = &types.Probe{}
		_ = json.Unmarshal([]byte(liveStr), svc.LivenessProbe)
	}
	if readyStr != "" && readyStr != "null" {
		svc.ReadinessProbe = &types.Probe{}
		_ = json.Unmarshal([]byte(readyStr), svc.ReadinessProbe)
	}
	if autoStr != "" && autoStr != "null" {
		svc.Autoscale = &types.AutoScaleConfig{}
		_ = json.Unmarshal([]byte(autoStr), svc.Autoscale)
	}
	svc.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	svc.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	return &svc, nil
}

// Replicas

func (s *SQLiteStorage) UpsertReplica(ctx context.Context, r *types.Replica) error {
	var readyInt int
	if r.Ready {
		readyInt = 1
	}

	var lastReschedStr, startedStr *string
	if r.LastRescheduledAt != nil {
		t := r.LastRescheduledAt.Format(time.RFC3339)
		lastReschedStr = &t
	}
	if r.StartedAt != nil {
		t := r.StartedAt.Format(time.RFC3339)
		startedStr = &t
	}

	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}

	query := `
	INSERT INTO replicas (
		id, service_name, node_name, container_id, host_port, status, health, ready,
		restart_count, reschedule_count, last_rescheduled_at, exit_code, message, created_at, started_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		service_name=excluded.service_name,
		node_name=excluded.node_name,
		container_id=excluded.container_id,
		host_port=excluded.host_port,
		status=excluded.status,
		health=excluded.health,
		ready=excluded.ready,
		restart_count=excluded.restart_count,
		reschedule_count=excluded.reschedule_count,
		last_rescheduled_at=excluded.last_rescheduled_at,
		exit_code=excluded.exit_code,
		message=excluded.message,
		started_at=excluded.started_at;
	`
	_, err := s.db.ExecContext(ctx, query,
		r.ID, r.ServiceName, r.NodeName, r.ContainerID, r.HostPort,
		string(r.Status), string(r.Health), readyInt, r.RestartCount,
		r.RescheduleCount, lastReschedStr, r.ExitCode, r.Message,
		r.CreatedAt.Format(time.RFC3339), startedStr,
	)
	return err
}

func (s *SQLiteStorage) ListReplicas(ctx context.Context) ([]*types.Replica, error) {
	query := `SELECT id, service_name, node_name, container_id, host_port, status, health, ready,
		restart_count, reschedule_count, last_rescheduled_at, exit_code, message, created_at, started_at FROM replicas`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*types.Replica
	for rows.Next() {
		r, err := scanReplica(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (s *SQLiteStorage) GetReplica(ctx context.Context, id string) (*types.Replica, error) {
	query := `SELECT id, service_name, node_name, container_id, host_port, status, health, ready,
		restart_count, reschedule_count, last_rescheduled_at, exit_code, message, created_at, started_at FROM replicas WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	r, err := scanReplica(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (s *SQLiteStorage) DeleteReplica(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM replicas WHERE id = ?", id)
	return err
}

func scanReplica(scanner interface {
	Scan(dest ...any) error
}) (*types.Replica, error) {
	var r types.Replica
	var readyInt int
	var statusStr, healthStr, createdStr string
	var lastReschedStr, startedStr sql.NullString

	err := scanner.Scan(
		&r.ID, &r.ServiceName, &r.NodeName, &r.ContainerID, &r.HostPort,
		&statusStr, &healthStr, &readyInt, &r.RestartCount, &r.RescheduleCount,
		&lastReschedStr, &r.ExitCode, &r.Message, &createdStr, &startedStr,
	)
	if err != nil {
		return nil, err
	}

	r.Status = types.PodStatus(statusStr)
	r.Health = types.HealthStatus(healthStr)
	r.Ready = readyInt == 1
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)

	if lastReschedStr.Valid {
		t, _ := time.Parse(time.RFC3339, lastReschedStr.String)
		r.LastRescheduledAt = &t
	}
	if startedStr.Valid {
		t, _ := time.Parse(time.RFC3339, startedStr.String)
		r.StartedAt = &t
	}

	return &r, nil
}

// Nodes

func (s *SQLiteStorage) UpsertNode(ctx context.Context, node *types.Node) error {
	var dockerOKInt int
	if node.DockerOK {
		dockerOKInt = 1
	}

	var notReadyStr *string
	if node.NotReadySince != nil {
		t := node.NotReadySince.Format(time.RFC3339)
		notReadyStr = &t
	}
	if node.LastSeen.IsZero() {
		node.LastSeen = time.Now().UTC()
	}

	query := `
	INSERT INTO nodes (
		name, address, capacity_cpu_millis, capacity_memory_bytes, status, last_seen, not_ready_since, docker_ok
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		address=excluded.address,
		capacity_cpu_millis=excluded.capacity_cpu_millis,
		capacity_memory_bytes=excluded.capacity_memory_bytes,
		status=excluded.status,
		last_seen=excluded.last_seen,
		not_ready_since=excluded.not_ready_since,
		docker_ok=excluded.docker_ok;
	`
	_, err := s.db.ExecContext(ctx, query,
		node.Name, node.Address, node.CapacityCPUMillis, node.CapacityMemoryBytes,
		node.Status, node.LastSeen.Format(time.RFC3339), notReadyStr, dockerOKInt,
	)
	return err
}

func (s *SQLiteStorage) ListNodes(ctx context.Context) ([]*types.Node, error) {
	query := `SELECT name, address, capacity_cpu_millis, capacity_memory_bytes, status, last_seen, not_ready_since, docker_ok FROM nodes`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*types.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, nil
}

func (s *SQLiteStorage) GetNode(ctx context.Context, name string) (*types.Node, error) {
	query := `SELECT name, address, capacity_cpu_millis, capacity_memory_bytes, status, last_seen, not_ready_since, docker_ok FROM nodes WHERE name = ?`
	row := s.db.QueryRowContext(ctx, query, name)
	n, err := scanNode(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

func (s *SQLiteStorage) MarkNodeReady(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE nodes SET status = 'Ready', last_seen = ?, not_ready_since = NULL WHERE name = ?`
	_, err := s.db.ExecContext(ctx, query, now, name)
	return err
}

func (s *SQLiteStorage) MarkNodeNotReady(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE nodes SET status = 'NotReady', not_ready_since = COALESCE(not_ready_since, ?) WHERE name = ?`
	_, err := s.db.ExecContext(ctx, query, now, name)
	return err
}

func (s *SQLiteStorage) RecordHeartbeat(ctx context.Context, name string, dockerOK bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	dockerOKInt := 0
	if dockerOK {
		dockerOKInt = 1
	}
	query := `UPDATE nodes SET last_seen = ?, status = 'Ready', not_ready_since = NULL, docker_ok = ? WHERE name = ?`
	_, err := s.db.ExecContext(ctx, query, now, dockerOKInt, name)
	return err
}

func scanNode(scanner interface {
	Scan(dest ...any) error
}) (*types.Node, error) {
	var n types.Node
	var dockerOKInt int
	var lastSeenStr string
	var notReadyStr sql.NullString

	err := scanner.Scan(
		&n.Name, &n.Address, &n.CapacityCPUMillis, &n.CapacityMemoryBytes,
		&n.Status, &lastSeenStr, &notReadyStr, &dockerOKInt,
	)
	if err != nil {
		return nil, err
	}

	n.DockerOK = dockerOKInt == 1
	n.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
	if notReadyStr.Valid {
		t, _ := time.Parse(time.RFC3339, notReadyStr.String)
		n.NotReadySince = &t
	}

	return &n, nil
}

// Metrics

func (s *SQLiteStorage) InsertMetric(ctx context.Context, m *types.Metrics) error {
	tsStr := m.Timestamp.UTC().Format(time.RFC3339)
	query := `INSERT INTO metrics (replica_id, ts, cpu_cores, memory_bytes) VALUES (?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, m.ReplicaID, tsStr, m.CPUCores, m.MemoryBytes)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		m.ID = id
	}
	return nil
}

func (s *SQLiteStorage) PruneOldMetrics(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339)
	query := `DELETE FROM metrics WHERE ts < ?`
	_, err := s.db.ExecContext(ctx, query, cutoff)
	return err
}

// Config / Token

func (s *SQLiteStorage) GetToken(ctx context.Context) (string, error) {
	query := `SELECT value FROM config WHERE key = 'api_token'`
	var token string
	err := s.db.QueryRowContext(ctx, query).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return token, err
}

func (s *SQLiteStorage) SetToken(ctx context.Context, token string) error {
	query := `INSERT INTO config (key, value) VALUES ('api_token', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`
	_, err := s.db.ExecContext(ctx, query, token)
	return err
}

