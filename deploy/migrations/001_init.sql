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
