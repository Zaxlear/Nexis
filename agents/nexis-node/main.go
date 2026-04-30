package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"nexis/packages/configlite"
	"nexis/packages/proto"
	"nexis/packages/sysstats"
	"nexis/packages/tcpprobe"
	"nexis/packages/wslite"
)

type nodeConfig struct {
	ConfigPath   string
	ID           string
	Name         string
	Token        string
	ControlURL   string
	ListenerHost string
	ListenerPort int
	PublicIPv4   string
	PublicIPv6   string
	Region       string
}

func main() {
	cfg := loadNodeConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startTCPListener(cfg)
	collector := sysstats.New()
	runControlLoop(ctx, cfg, collector)
}

func loadNodeConfig() nodeConfig {
	configPath := env("NEXIS_CONFIG", "")
	if argPath := configlite.PathFromArgs(os.Args[1:], "config"); argPath != "" {
		configPath = argPath
	}
	fileValues, err := configlite.Load(configPath)
	if err != nil {
		log.Fatalf("load config %s failed: %v", configPath, err)
	}
	cfg := nodeConfig{
		ConfigPath:   configPath,
		ID:           env("NEXIS_NODE_ID", fileValues.String("agent.id", "node-local-oracle")),
		Name:         env("NEXIS_NODE_NAME", fileValues.String("agent.name", "Oracle")),
		Token:        env("NEXIS_NODE_TOKEN", fileValues.String("agent.token", "nexis-local-token")),
		ControlURL:   env("NEXIS_CONTROL_URL", fileValues.String("console.control_url", "ws://127.0.0.1:47151/ws")),
		ListenerHost: env("NEXIS_NODE_LISTENER_HOST", fileValues.String("listener.host", "0.0.0.0")),
		ListenerPort: envInt("NEXIS_NODE_LISTENER_PORT", fileValues.Int("listener.port", 47211)),
		PublicIPv4:   env("NEXIS_NODE_PUBLIC_IPV4", fileValues.String("agent.public_ipv4", "127.0.0.1")),
		PublicIPv6:   env("NEXIS_NODE_PUBLIC_IPV6", fileValues.String("agent.public_ipv6", "")),
		Region:       env("NEXIS_NODE_REGION", fileValues.String("agent.region", "local")),
	}
	flag.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "YAML config file path")
	flag.StringVar(&cfg.ID, "id", cfg.ID, "agent id")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "node name")
	flag.StringVar(&cfg.Token, "token", cfg.Token, "agent token")
	flag.StringVar(&cfg.ControlURL, "control-url", cfg.ControlURL, "Nexis Console control URL")
	flag.StringVar(&cfg.ListenerHost, "listener-host", cfg.ListenerHost, "TCP probe listener host")
	flag.IntVar(&cfg.ListenerPort, "listener-port", cfg.ListenerPort, "TCP probe listener port")
	flag.StringVar(&cfg.PublicIPv4, "public-ipv4", cfg.PublicIPv4, "public IPv4 or test host")
	flag.StringVar(&cfg.PublicIPv6, "public-ipv6", cfg.PublicIPv6, "public IPv6")
	flag.StringVar(&cfg.Region, "region", cfg.Region, "node region")
	flag.Parse()
	return cfg
}

func startTCPListener(cfg nodeConfig) {
	addr := fmt.Sprintf("%s:%d", cfg.ListenerHost, cfg.ListenerPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Nexis Node TCP test listener failed on %s: %v", addr, err)
	}
	log.Printf("Nexis Node TCP test listener: %s", addr)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("TCP listener accept failed: %v", err)
				return
			}
			go tcpprobe.HandleEchoConn(conn)
		}
	}()
}

func runControlLoop(ctx context.Context, cfg nodeConfig, collector *sysstats.Collector) {
	backoff := time.Second
	for ctx.Err() == nil {
		conn, err := wslite.Dial(ctx, cfg.ControlURL)
		if err != nil {
			sleepBackoff(ctx, &backoff, err)
			continue
		}
		backoff = time.Second
		if err := runControlSession(ctx, cfg, collector, conn); err != nil && ctx.Err() == nil {
			log.Printf("control session ended: %v", err)
		}
	}
}

func runControlSession(ctx context.Context, cfg nodeConfig, collector *sysstats.Collector, conn *wslite.Conn) error {
	stats := collector.Snapshot()
	register := proto.ControlMessage{
		Type:      "register",
		Component: proto.ComponentNexisNode,
		AgentID:   cfg.ID,
		Name:      cfg.Name,
		Token:     cfg.Token,
		Version:   "0.1.0",
		Capabilities: []string{
			"tcp-test-listener",
			"reverse-logical-runner",
			"resource-reporter",
		},
		Meta: map[string]any{
			"public_ipv4":    cfg.PublicIPv4,
			"public_ipv6":    cfg.PublicIPv6,
			"test_host":      cfg.PublicIPv4,
			"listener_port":  cfg.ListenerPort,
			"region":         cfg.Region,
			"os":             runtime.GOOS,
			"arch":           runtime.GOARCH,
			"virtualization": stats.Virtualization,
		},
	}
	if err := conn.WriteJSON(register); err != nil {
		_ = conn.Close()
		return err
	}
	var ack proto.ControlMessage
	if err := conn.ReadJSON(&ack); err != nil {
		_ = conn.Close()
		return err
	}
	if ack.Type != "register_ack" {
		_ = conn.Close()
		return fmt.Errorf("register failed: %s", ack.Error)
	}
	log.Printf("registered to Nexis Console as %s", ack.SessionID)

	heartbeatSec := ack.HeartbeatIntervalSec
	if heartbeatSec <= 0 {
		heartbeatSec = 15
	}
	heartbeatDone := make(chan struct{})
	go heartbeatLoop(conn, heartbeatDone, ack.SessionID, collector, time.Duration(heartbeatSec)*time.Second)
	defer close(heartbeatDone)

	for {
		var msg proto.ControlMessage
		if err := conn.ReadJSON(&msg); err != nil {
			_ = conn.Close()
			return err
		}
		if msg.Type == "run_test" && msg.TestType == proto.TestReverseLogicalTCP {
			go runReverseTest(ctx, conn, msg)
		}
	}
}

func heartbeatLoop(conn *wslite.Conn, done <-chan struct{}, sessionID string, collector *sysstats.Collector, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			stats := collector.Snapshot()
			msg := proto.ControlMessage{
				Type:      "heartbeat",
				SessionID: sessionID,
				TS:        time.Now().UnixMilli(),
				Stats:     &stats,
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

func runReverseTest(ctx context.Context, conn *wslite.Conn, msg proto.ControlMessage) {
	if msg.Target == nil {
		return
	}
	params := proto.DefaultTestParams()
	if msg.Params != nil {
		params = *msg.Params
	}
	testCtx, cancel := context.WithTimeout(ctx, time.Duration(params.ConnectTimeoutMS+params.ReadTimeoutMS*params.ProbeCount+3000)*time.Millisecond)
	defer cancel()

	result, raw, err := tcpprobe.Run(testCtx, *msg.Target, params, msg.JobID)
	if err != nil {
		raw["error"] = err.Error()
	}
	report := proto.ControlMessage{
		Type:      "test_result",
		JobID:     msg.JobID,
		TestType:  proto.TestReverseLogicalTCP,
		Direction: proto.DirectionReverse,
		PathMode:  proto.PathModeTunnel,
		NodeID:    msg.NodeID,
		ProbeID:   msg.ProbeID,
		Result:    &result,
		Raw:       raw,
	}
	if err := conn.WriteJSON(report); err != nil {
		log.Printf("reverse test report failed: %v", err)
	}
}

func sleepBackoff(ctx context.Context, backoff *time.Duration, cause error) {
	jitter := time.Duration(rand.Int63n(int64(*backoff / 5)))
	wait := *backoff + jitter
	log.Printf("connect failed: %v; retry in %s", cause, wait)
	timer := time.NewTimer(wait)
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	timer.Stop()
	*backoff *= 2
	if *backoff > 30*time.Second {
		*backoff = 30 * time.Second
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
