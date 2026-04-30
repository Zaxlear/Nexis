package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type postgresStore struct {
	db *sql.DB
}

func openPostgresStore(cfg config) (*postgresStore, error) {
	db, err := sql.Open("postgres", cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &postgresStore{db: db}
	if cfg.DatabaseAutoMigrate {
		if err := store.migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if err := store.closePreviousSessions(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (p *postgresStore) migrate() error {
	_, err := p.db.Exec(`
CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  component_type TEXT NOT NULL CHECK (component_type IN ('node', 'probe')),
  name TEXT NOT NULL,
  region TEXT,
  isp TEXT,
  network_stack TEXT,
  public_ipv4 TEXT,
  public_ipv6 TEXT,
  status TEXT NOT NULL DEFAULT 'offline',
  auth_token_hash TEXT NOT NULL,
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  meta JSONB DEFAULT '{}'::jsonb,
  reverse_port INTEGER
);

CREATE TABLE IF NOT EXISTS agent_sessions (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL REFERENCES agents(id),
  component_type TEXT NOT NULL CHECK (component_type IN ('node', 'probe')),
  connected_at TIMESTAMPTZ NOT NULL,
  last_heartbeat_at TIMESTAMPTZ,
  lease_expires_at TIMESTAMPTZ,
  disconnected_at TIMESTAMPTZ,
  is_active BOOLEAN NOT NULL DEFAULT true,
  close_reason TEXT,
  remote_addr TEXT,
  metadata JSONB DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS node_system_snapshots (
  id BIGSERIAL PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES agents(id),
  ts TIMESTAMPTZ NOT NULL,
  cpu_usage_percent DOUBLE PRECISION,
  cpu_cores INTEGER,
  memory_total_mb DOUBLE PRECISION,
  memory_used_mb DOUBLE PRECISION,
  disk_total_gb DOUBLE PRECISION,
  disk_used_gb DOUBLE PRECISION,
  network_rx_bytes BIGINT,
  network_tx_bytes BIGINT,
  uptime_seconds BIGINT,
  os TEXT,
  arch TEXT,
  virtualization TEXT
);
CREATE INDEX IF NOT EXISTS idx_node_system_snapshots_node_ts ON node_system_snapshots(node_id, ts DESC);

CREATE TABLE IF NOT EXISTS tcp_test_results (
  id BIGSERIAL PRIMARY KEY,
  ts TIMESTAMPTZ NOT NULL,
  node_id TEXT NOT NULL REFERENCES agents(id),
  probe_id TEXT NOT NULL REFERENCES agents(id),
  test_type TEXT NOT NULL CHECK (test_type IN ('forward_direct_tcp', 'reverse_logical_tcp')),
  direction TEXT NOT NULL CHECK (direction IN ('forward', 'reverse')),
  path_mode TEXT NOT NULL CHECK (path_mode IN ('direct', 'tunnel')),
  connect_latency_ms DOUBLE PRECISION,
  app_rtt_ms DOUBLE PRECISION,
  jitter_ms DOUBLE PRECISION,
  success_rate DOUBLE PRECISION,
  timeout_rate DOUBLE PRECISION,
  probe_loss_rate DOUBLE PRECISION,
  tcp_retransmissions INTEGER,
  sample_count INTEGER,
  raw JSONB DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_tcp_test_results_node_ts ON tcp_test_results(node_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_tcp_test_results_node_probe_direction_ts ON tcp_test_results(node_id, probe_id, direction, ts DESC);

CREATE TABLE IF NOT EXISTS reverse_tunnel_ports (
  id BIGSERIAL PRIMARY KEY,
  probe_id TEXT NOT NULL REFERENCES agents(id),
  port INTEGER NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'assigned',
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  released_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_reverse_tunnel_ports_probe ON reverse_tunnel_ports(probe_id);
`)
	return err
}

func (p *postgresStore) closePreviousSessions() error {
	_, err := p.db.Exec(`
UPDATE agent_sessions
SET is_active = false,
    disconnected_at = COALESCE(disconnected_at, now()),
    close_reason = COALESCE(NULLIF(close_reason, ''), 'console_restart')
WHERE is_active = true`)
	return err
}

func (s *store) loadFromDatabase() error {
	if s.db == nil {
		return nil
	}
	if err := s.loadAgentsFromDatabase(); err != nil {
		return err
	}
	if err := s.loadSnapshotsFromDatabase(); err != nil {
		return err
	}
	if err := s.loadResultsFromDatabase(); err != nil {
		return err
	}
	return s.loadReversePortsFromDatabase()
}

func (s *store) loadAgentsFromDatabase() error {
	rows, err := s.db.db.Query(`
SELECT id, component_type, name, COALESCE(region, ''), COALESCE(isp, ''), COALESCE(network_stack, ''),
       COALESCE(public_ipv4, ''), COALESCE(public_ipv6, ''), status, auth_token_hash, last_seen_at,
       created_at, updated_at, COALESCE(meta, '{}'::jsonb), COALESCE(reverse_port, 0)
FROM agents`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item agent
		var metaRaw []byte
		var lastSeen sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.ComponentType,
			&item.Name,
			&item.Region,
			&item.ISP,
			&item.NetworkStack,
			&item.PublicIPv4,
			&item.PublicIPv6,
			&item.Status,
			&item.TokenHash,
			&lastSeen,
			&item.CreatedAt,
			&item.UpdatedAt,
			&metaRaw,
			&item.ReversePort,
		); err != nil {
			return err
		}
		if lastSeen.Valid {
			item.LastSeenAt = &lastSeen.Time
		}
		item.Meta = map[string]any{}
		_ = json.Unmarshal(metaRaw, &item.Meta)
		s.agents[item.ID] = &item
	}
	return rows.Err()
}

func (s *store) loadSnapshotsFromDatabase() error {
	rows, err := s.db.db.Query(`
SELECT id, node_id, ts, cpu_usage_percent, cpu_cores, memory_total_mb, memory_used_mb,
       disk_total_gb, disk_used_gb, network_rx_bytes, network_tx_bytes, uptime_seconds,
       COALESCE(os, ''), COALESCE(arch, ''), COALESCE(virtualization, '')
FROM node_system_snapshots
WHERE ts >= now() - interval '8 days'
ORDER BY id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item systemSnapshot
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.TS,
			&item.Stats.CPUUsagePercent,
			&item.Stats.CPUCores,
			&item.Stats.MemoryTotalMB,
			&item.Stats.MemoryUsedMB,
			&item.Stats.DiskTotalGB,
			&item.Stats.DiskUsedGB,
			&item.Stats.NetworkRXBytes,
			&item.Stats.NetworkTXBytes,
			&item.Stats.UptimeSeconds,
			&item.Stats.OS,
			&item.Stats.Arch,
			&item.Stats.Virtualization,
		); err != nil {
			return err
		}
		s.snapshots = append(s.snapshots, item)
		if item.ID > s.nextSnapshotID {
			s.nextSnapshotID = item.ID
		}
		if agent := s.agents[item.NodeID]; agent != nil {
			stats := item.Stats
			agent.LatestStats = &stats
		}
	}
	return rows.Err()
}

func (s *store) loadResultsFromDatabase() error {
	rows, err := s.db.db.Query(`
SELECT id, ts, node_id, probe_id, test_type, direction, path_mode,
       connect_latency_ms, app_rtt_ms, jitter_ms, success_rate, timeout_rate,
       probe_loss_rate, tcp_retransmissions, sample_count, COALESCE(raw, '{}'::jsonb)
FROM tcp_test_results
WHERE ts >= now() - interval '8 days'
ORDER BY id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item tcpResult
		var raw []byte
		if err := rows.Scan(
			&item.ID,
			&item.TS,
			&item.NodeID,
			&item.ProbeID,
			&item.TestType,
			&item.Direction,
			&item.PathMode,
			&item.Result.ConnectLatencyMS,
			&item.Result.AppRTTMS,
			&item.Result.JitterMS,
			&item.Result.SuccessRate,
			&item.Result.TimeoutRate,
			&item.Result.ProbeLossRate,
			&item.Result.TCPRetransmissions,
			&item.Result.SampleCount,
			&raw,
		); err != nil {
			return err
		}
		item.Raw = map[string]any{}
		_ = json.Unmarshal(raw, &item.Raw)
		s.results = append(s.results, item)
		if item.ID > s.nextResultID {
			s.nextResultID = item.ID
		}
	}
	return rows.Err()
}

func (s *store) loadReversePortsFromDatabase() error {
	rows, err := s.db.db.Query(`SELECT probe_id, port FROM reverse_tunnel_ports WHERE released_at IS NULL AND status = 'assigned'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var probeID string
		var port int
		if err := rows.Scan(&probeID, &port); err != nil {
			return err
		}
		s.reversePortToProbe[port] = probeID
		if port >= s.nextReversePort {
			s.nextReversePort = port + 1
		}
		if item := s.agents[probeID]; item != nil {
			item.ReversePort = port
		}
	}
	return rows.Err()
}

func (s *store) persistAgent(item *agent) {
	if s.db == nil || item == nil {
		return
	}
	if err := s.db.upsertAgent(item); err != nil {
		log.Printf("persist agent %s failed: %v", item.ID, err)
	}
}

func (p *postgresStore) upsertAgent(item *agent) error {
	meta, _ := json.Marshal(item.Meta)
	var lastSeen any
	if item.LastSeenAt != nil {
		lastSeen = *item.LastSeenAt
	}
	_, err := p.db.Exec(`
INSERT INTO agents (
  id, component_type, name, region, isp, network_stack, public_ipv4, public_ipv6,
  status, auth_token_hash, last_seen_at, created_at, updated_at, meta, reverse_port
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (id) DO UPDATE SET
  component_type = EXCLUDED.component_type,
  name = EXCLUDED.name,
  region = EXCLUDED.region,
  isp = EXCLUDED.isp,
  network_stack = EXCLUDED.network_stack,
  public_ipv4 = EXCLUDED.public_ipv4,
  public_ipv6 = EXCLUDED.public_ipv6,
  status = EXCLUDED.status,
  auth_token_hash = EXCLUDED.auth_token_hash,
  last_seen_at = EXCLUDED.last_seen_at,
  updated_at = EXCLUDED.updated_at,
  meta = EXCLUDED.meta,
  reverse_port = EXCLUDED.reverse_port`,
		item.ID,
		item.ComponentType,
		item.Name,
		item.Region,
		item.ISP,
		item.NetworkStack,
		item.PublicIPv4,
		item.PublicIPv6,
		item.Status,
		item.TokenHash,
		lastSeen,
		item.CreatedAt,
		item.UpdatedAt,
		string(meta),
		nullInt(item.ReversePort),
	)
	return err
}

func (s *store) persistSession(sess *session) {
	if s.db == nil || sess == nil {
		return
	}
	if err := s.db.upsertSession(sess); err != nil {
		log.Printf("persist session %s failed: %v", sess.ID, err)
	}
}

func (p *postgresStore) upsertSession(sess *session) error {
	var disconnected any
	if sess.DisconnectedAt != nil {
		disconnected = *sess.DisconnectedAt
	}
	_, err := p.db.Exec(`
INSERT INTO agent_sessions (
  id, agent_id, component_type, connected_at, last_heartbeat_at, lease_expires_at,
  disconnected_at, is_active, close_reason, remote_addr, metadata
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'{}'::jsonb)
ON CONFLICT (id) DO UPDATE SET
  last_heartbeat_at = EXCLUDED.last_heartbeat_at,
  lease_expires_at = EXCLUDED.lease_expires_at,
  disconnected_at = EXCLUDED.disconnected_at,
  is_active = EXCLUDED.is_active,
  close_reason = EXCLUDED.close_reason`,
		sess.ID,
		sess.AgentID,
		sess.ComponentType,
		sess.ConnectedAt,
		sess.LastHeartbeat,
		sess.LeaseExpires,
		disconnected,
		sess.IsActive,
		sess.CloseReason,
		sess.RemoteAddr,
	)
	return err
}

func (s *store) persistSnapshot(snapshot systemSnapshot) {
	if s.db == nil {
		return
	}
	id, err := s.db.insertSnapshot(snapshot)
	if err != nil {
		log.Printf("persist snapshot failed: %v", err)
		return
	}
	if id > 0 {
		s.mu.Lock()
		if id > s.nextSnapshotID {
			s.nextSnapshotID = id
		}
		s.mu.Unlock()
	}
}

func (p *postgresStore) insertSnapshot(snapshot systemSnapshot) (int64, error) {
	var id int64
	err := p.db.QueryRow(`
INSERT INTO node_system_snapshots (
  node_id, ts, cpu_usage_percent, cpu_cores, memory_total_mb, memory_used_mb,
  disk_total_gb, disk_used_gb, network_rx_bytes, network_tx_bytes, uptime_seconds,
  os, arch, virtualization
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
RETURNING id`,
		snapshot.NodeID,
		snapshot.TS,
		snapshot.Stats.CPUUsagePercent,
		snapshot.Stats.CPUCores,
		snapshot.Stats.MemoryTotalMB,
		snapshot.Stats.MemoryUsedMB,
		snapshot.Stats.DiskTotalGB,
		snapshot.Stats.DiskUsedGB,
		snapshot.Stats.NetworkRXBytes,
		snapshot.Stats.NetworkTXBytes,
		snapshot.Stats.UptimeSeconds,
		snapshot.Stats.OS,
		snapshot.Stats.Arch,
		snapshot.Stats.Virtualization,
	).Scan(&id)
	return id, err
}

func (s *store) persistResult(result tcpResult) {
	if s.db == nil {
		return
	}
	id, err := s.db.insertResult(result)
	if err != nil {
		log.Printf("persist tcp result failed: %v", err)
		return
	}
	if id > 0 {
		s.mu.Lock()
		if id > s.nextResultID {
			s.nextResultID = id
		}
		s.mu.Unlock()
	}
}

func (p *postgresStore) insertResult(result tcpResult) (int64, error) {
	raw, _ := json.Marshal(result.Raw)
	var id int64
	err := p.db.QueryRow(`
INSERT INTO tcp_test_results (
  ts, node_id, probe_id, test_type, direction, path_mode, connect_latency_ms,
  app_rtt_ms, jitter_ms, success_rate, timeout_rate, probe_loss_rate,
  tcp_retransmissions, sample_count, raw
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING id`,
		result.TS,
		result.NodeID,
		result.ProbeID,
		result.TestType,
		result.Direction,
		result.PathMode,
		result.Result.ConnectLatencyMS,
		result.Result.AppRTTMS,
		result.Result.JitterMS,
		result.Result.SuccessRate,
		result.Result.TimeoutRate,
		result.Result.ProbeLossRate,
		result.Result.TCPRetransmissions,
		result.Result.SampleCount,
		string(raw),
	).Scan(&id)
	return id, err
}

func (s *store) persistReversePort(probeID string, port int) {
	if s.db == nil || probeID == "" || port == 0 {
		return
	}
	if err := s.db.upsertReversePort(probeID, port); err != nil {
		log.Printf("persist reverse port failed: %v", err)
	}
}

func (p *postgresStore) upsertReversePort(probeID string, port int) error {
	_, err := p.db.Exec(`
INSERT INTO reverse_tunnel_ports (probe_id, port, status, assigned_at)
VALUES ($1, $2, 'assigned', now())
ON CONFLICT (port) DO UPDATE SET
  probe_id = EXCLUDED.probe_id,
  status = 'assigned',
  released_at = NULL`,
		probeID,
		port,
	)
	return err
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *store) hasPersistedDemoData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.agents["demo-node-oracle"]
	return ok
}

func (s *store) databaseMode() string {
	if s.db == nil {
		return "memory"
	}
	return "postgres"
}
