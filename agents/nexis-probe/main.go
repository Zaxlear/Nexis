package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"nexis/packages/configlite"
	"nexis/packages/proto"
	"nexis/packages/sysstats"
	"nexis/packages/tcpprobe"
	"nexis/packages/wslite"
)

type probeConfig struct {
	ConfigPath  string
	ID          string
	Name        string
	Token       string
	ControlURL  string
	Region      string
	ISP         string
	Stack       string
	Concurrency int
}

type reverseResponder struct {
	mu      sync.Mutex
	buffers map[string][]byte
}

func main() {
	cfg := loadProbeConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	collector := sysstats.New()
	runControlLoop(ctx, cfg, collector)
}

func loadProbeConfig() probeConfig {
	configPath := env("NEXIS_CONFIG", "")
	if argPath := configlite.PathFromArgs(os.Args[1:], "config"); argPath != "" {
		configPath = argPath
	}
	fileValues, err := configlite.Load(configPath)
	if err != nil {
		log.Fatalf("load config %s failed: %v", configPath, err)
	}
	cfg := probeConfig{
		ConfigPath:  configPath,
		ID:          env("NEXIS_PROBE_ID", fileValues.String("agent.id", "probe-local-guangdong-ct-v4")),
		Name:        env("NEXIS_PROBE_NAME", fileValues.String("agent.name", "广东电信IPv4")),
		Token:       env("NEXIS_PROBE_TOKEN", fileValues.String("agent.token", "nexis-local-token")),
		ControlURL:  env("NEXIS_CONTROL_URL", fileValues.String("console.control_url", "ws://127.0.0.1:47151/ws")),
		Region:      env("NEXIS_PROBE_REGION", fileValues.String("agent.region", "广东")),
		ISP:         env("NEXIS_PROBE_ISP", fileValues.String("agent.isp", "电信")),
		Stack:       env("NEXIS_PROBE_STACK", fileValues.String("agent.network_stack", "IPv4")),
		Concurrency: envInt("NEXIS_PROBE_CONCURRENCY", fileValues.Int("probe.concurrency", 4)),
	}
	flag.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "YAML config file path")
	flag.StringVar(&cfg.ID, "id", cfg.ID, "agent id")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "probe name")
	flag.StringVar(&cfg.Token, "token", cfg.Token, "agent token")
	flag.StringVar(&cfg.ControlURL, "control-url", cfg.ControlURL, "Nexis Console control URL")
	flag.StringVar(&cfg.Region, "region", cfg.Region, "probe region")
	flag.StringVar(&cfg.ISP, "isp", cfg.ISP, "probe ISP")
	flag.StringVar(&cfg.Stack, "stack", cfg.Stack, "IPv4 or IPv6")
	flag.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "global probe concurrency")
	flag.Parse()
	return cfg
}

func runControlLoop(ctx context.Context, cfg probeConfig, collector *sysstats.Collector) {
	backoff := time.Second
	for ctx.Err() == nil {
		conn, err := wslite.Dial(ctx, cfg.ControlURL)
		if err != nil {
			sleepBackoff(ctx, &backoff, err)
			continue
		}
		connectedAt := time.Now()
		if err := runControlSession(ctx, cfg, collector, conn); err != nil && ctx.Err() == nil {
			log.Printf("control session ended: %v", err)
			if time.Since(connectedAt) < 10*time.Second {
				sleepBackoff(ctx, &backoff, err)
				continue
			}
		}
		backoff = time.Second
	}
}

func runControlSession(ctx context.Context, cfg probeConfig, collector *sysstats.Collector, conn *wslite.Conn) error {
	register := proto.ControlMessage{
		Type:      "register",
		Component: proto.ComponentNexisProbe,
		AgentID:   cfg.ID,
		Name:      cfg.Name,
		Token:     cfg.Token,
		Version:   "0.1.0",
		Capabilities: []string{
			"tcp-forward-probe",
			"reverse-logical-responder",
		},
		Meta: map[string]any{
			"region":        cfg.Region,
			"isp":           cfg.ISP,
			"network_stack": cfg.Stack,
			"os":            runtime.GOOS,
			"arch":          runtime.GOARCH,
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
	if ack.ReverseTunnel != nil && ack.ReverseTunnel.Enabled {
		log.Printf("registered to Nexis Console as %s; reverse logical port %d", ack.SessionID, ack.ReverseTunnel.ListenPort)
	} else {
		log.Printf("registered to Nexis Console as %s", ack.SessionID)
	}

	heartbeatSec := ack.HeartbeatIntervalSec
	if heartbeatSec <= 0 {
		heartbeatSec = 15
	}
	heartbeatDone := make(chan struct{})
	go heartbeatLoop(conn, heartbeatDone, ack.SessionID, collector, time.Duration(heartbeatSec)*time.Second)
	defer close(heartbeatDone)

	sem := make(chan struct{}, cfg.Concurrency)
	responder := &reverseResponder{buffers: make(map[string][]byte)}
	for {
		var msg proto.ControlMessage
		if err := conn.ReadJSON(&msg); err != nil {
			_ = conn.Close()
			return err
		}
		switch msg.Type {
		case "run_test":
			if msg.TestType == proto.TestForwardDirectTCP {
				go runForwardTest(ctx, conn, sem, msg)
			}
		case "reverse_stream_open":
			responder.open(msg.StreamID)
		case "reverse_stream_data":
			responder.data(conn, msg.StreamID, msg.Data)
		case "reverse_stream_close":
			responder.close(msg.StreamID)
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

func runForwardTest(ctx context.Context, conn *wslite.Conn, sem chan struct{}, msg proto.ControlMessage) {
	if msg.Target == nil {
		return
	}
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
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
		TestType:  proto.TestForwardDirectTCP,
		Direction: proto.DirectionForward,
		PathMode:  proto.PathModeDirect,
		NodeID:    msg.NodeID,
		ProbeID:   msg.ProbeID,
		Result:    &result,
		Raw:       raw,
	}
	if err := conn.WriteJSON(report); err != nil {
		log.Printf("forward test report failed: %v", err)
	}
}

func (r *reverseResponder) open(streamID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buffers[streamID] = nil
}

func (r *reverseResponder) close(streamID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buffers, streamID)
}

func (r *reverseResponder) data(conn *wslite.Conn, streamID, encoded string) {
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return
	}
	r.mu.Lock()
	buffer, ok := r.buffers[streamID]
	if !ok {
		r.mu.Unlock()
		return
	}
	buffer = append(buffer, payload...)
	var replies [][]byte
	for {
		index := bytes.IndexByte(buffer, '\n')
		if index < 0 {
			break
		}
		line := append([]byte(nil), buffer[:index+1]...)
		buffer = buffer[index+1:]
		if ack, ok := tcpprobe.AckForLine(line); ok {
			replies = append(replies, append(ack, '\n'))
		}
	}
	r.buffers[streamID] = buffer
	r.mu.Unlock()

	for _, reply := range replies {
		msg := proto.ControlMessage{
			Type:     "reverse_stream_data",
			StreamID: streamID,
			Data:     base64.StdEncoding.EncodeToString(reply),
		}
		if err := conn.WriteJSON(msg); err != nil {
			return
		}
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
