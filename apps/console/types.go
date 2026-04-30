package main

import (
	"net"
	"sync"
	"time"

	"nexis/packages/proto"
	"nexis/packages/wslite"
)

type config struct {
	ConfigPath          string
	WebAddr             string
	APIAddr             string
	ControlAddr         string
	MetricsAddr         string
	WebDir              string
	PublicHost          string
	ReverseStart        int
	ReverseEnd          int
	HeartbeatInterval   time.Duration
	HeartbeatTimeout    time.Duration
	LeaseTTL            time.Duration
	ScheduleInterval    time.Duration
	SeedDemo            bool
	AgentTokenHashes    map[string]string
	DatabaseDSN         string
	DatabaseAutoMigrate bool
}

type store struct {
	mu                 sync.RWMutex
	agents             map[string]*agent
	sessions           map[string]*session
	snapshots          []systemSnapshot
	results            []tcpResult
	streams            map[string]*reverseStream
	reverseListeners   map[int]bool
	reversePortToProbe map[int]string
	nextReversePort    int
	nextSnapshotID     int64
	nextResultID       int64
	db                 *postgresStore
}

type agent struct {
	ID            string
	ComponentType string
	Name          string
	Region        string
	ISP           string
	NetworkStack  string
	PublicIPv4    string
	PublicIPv6    string
	Status        string
	TokenHash     string
	LastSeenAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ReversePort   int
	Meta          map[string]any
	LatestStats   *proto.SystemStats
	SessionID     string
	Demo          bool
}

type session struct {
	ID             string
	AgentID        string
	ComponentType  string
	Conn           *wslite.Conn
	ConnectedAt    time.Time
	LastHeartbeat  time.Time
	LeaseExpires   time.Time
	DisconnectedAt *time.Time
	IsActive       bool
	CloseReason    string
	RemoteAddr     string
}

type systemSnapshot struct {
	ID     int64             `json:"id"`
	NodeID string            `json:"node_id"`
	TS     time.Time         `json:"ts"`
	Stats  proto.SystemStats `json:"stats"`
}

type tcpResult struct {
	ID        int64            `json:"id"`
	TS        time.Time        `json:"ts"`
	NodeID    string           `json:"node_id"`
	ProbeID   string           `json:"probe_id"`
	TestType  string           `json:"test_type"`
	Direction string           `json:"direction"`
	PathMode  string           `json:"path_mode"`
	Result    proto.TestResult `json:"result"`
	Raw       map[string]any   `json:"raw,omitempty"`
}

type reverseStream struct {
	ID        string
	ProbeID   string
	Conn      net.Conn
	CreatedAt time.Time
}

func newStore(reverseStart int) *store {
	return &store{
		agents:             make(map[string]*agent),
		sessions:           make(map[string]*session),
		streams:            make(map[string]*reverseStream),
		reverseListeners:   make(map[int]bool),
		reversePortToProbe: make(map[int]string),
		nextReversePort:    reverseStart,
	}
}
