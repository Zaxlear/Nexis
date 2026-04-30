package proto

import "time"

const (
	ComponentNexisNode  = "nexis-node"
	ComponentNexisProbe = "nexis-probe"

	AgentTypeNode  = "node"
	AgentTypeProbe = "probe"

	TestForwardDirectTCP  = "forward_direct_tcp"
	TestReverseLogicalTCP = "reverse_logical_tcp"

	DirectionForward = "forward"
	DirectionReverse = "reverse"

	PathModeDirect = "direct"
	PathModeTunnel = "tunnel"
)

type ControlMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`

	Component    string         `json:"component,omitempty"`
	AgentID      string         `json:"agent_id,omitempty"`
	Name         string         `json:"name,omitempty"`
	Token        string         `json:"token,omitempty"`
	Version      string         `json:"version,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`

	HeartbeatIntervalSec int            `json:"heartbeat_interval_sec,omitempty"`
	LeaseTTLSec          int            `json:"lease_ttl_sec,omitempty"`
	ServerTime           int64          `json:"server_time,omitempty"`
	ReverseTunnel        *ReverseTunnel `json:"reverse_tunnel,omitempty"`

	TS    int64        `json:"ts,omitempty"`
	Stats *SystemStats `json:"stats,omitempty"`

	JobID     string         `json:"job_id,omitempty"`
	TestType  string         `json:"test_type,omitempty"`
	Direction string         `json:"direction,omitempty"`
	PathMode  string         `json:"path_mode,omitempty"`
	NodeID    string         `json:"node_id,omitempty"`
	ProbeID   string         `json:"probe_id,omitempty"`
	Target    *TestTarget    `json:"target,omitempty"`
	Params    *TestParams    `json:"params,omitempty"`
	Result    *TestResult    `json:"result,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`

	StreamID string `json:"stream_id,omitempty"`
	Data     string `json:"data,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Error    string `json:"error,omitempty"`
}

type ReverseTunnel struct {
	Enabled    bool `json:"enabled"`
	ListenPort int  `json:"listen_port"`
}

type TestTarget struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TestParams struct {
	ProbeCount       int `json:"probe_count"`
	ConnectTimeoutMS int `json:"connect_timeout_ms"`
	ReadTimeoutMS    int `json:"read_timeout_ms"`
	WriteTimeoutMS   int `json:"write_timeout_ms"`
	IntervalMS       int `json:"interval_ms"`
}

func DefaultTestParams() TestParams {
	return TestParams{
		ProbeCount:       5,
		ConnectTimeoutMS: 3000,
		ReadTimeoutMS:    3000,
		WriteTimeoutMS:   3000,
		IntervalMS:       200,
	}
}

type TestResult struct {
	ConnectLatencyMS   float64 `json:"connect_latency_ms"`
	AppRTTMS           float64 `json:"app_rtt_ms"`
	JitterMS           float64 `json:"jitter_ms"`
	SuccessRate        float64 `json:"success_rate"`
	TimeoutRate        float64 `json:"timeout_rate"`
	ProbeLossRate      float64 `json:"probe_loss_rate"`
	TCPRetransmissions int     `json:"tcp_retransmissions"`
	SampleCount        int     `json:"sample_count"`
}

type SystemStats struct {
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	CPUCores        int     `json:"cpu_cores"`
	MemoryTotalMB   float64 `json:"memory_total_mb"`
	MemoryUsedMB    float64 `json:"memory_used_mb"`
	DiskTotalGB     float64 `json:"disk_total_gb"`
	DiskUsedGB      float64 `json:"disk_used_gb"`
	NetworkRXBytes  int64   `json:"network_rx_bytes"`
	NetworkTXBytes  int64   `json:"network_tx_bytes"`
	UptimeSeconds   int64   `json:"uptime_seconds"`
	OS              string  `json:"os"`
	Arch            string  `json:"arch"`
	Virtualization  string  `json:"virtualization"`
}

type AgentView struct {
	ID            string         `json:"id"`
	ComponentType string         `json:"component_type"`
	Name          string         `json:"name"`
	Region        string         `json:"region,omitempty"`
	ISP           string         `json:"isp,omitempty"`
	NetworkStack  string         `json:"network_stack,omitempty"`
	PublicIPv4    string         `json:"public_ipv4,omitempty"`
	PublicIPv6    string         `json:"public_ipv6,omitempty"`
	Status        string         `json:"status"`
	LastSeenAt    *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	ReversePort   int            `json:"reverse_port,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
	LatestStats   *SystemStats   `json:"latest_stats,omitempty"`
	Summary       map[string]any `json:"summary,omitempty"`
}
