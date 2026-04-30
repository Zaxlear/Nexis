package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"nexis/packages/proto"
	"nexis/packages/wslite"
)

func controlHandler(st *store, cfg config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wslite.AcceptHTTP(w, r)
		if err != nil {
			return
		}
		go st.handleControlConn(conn, cfg)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (s *store) handleControlConn(conn *wslite.Conn, cfg config) {
	var register proto.ControlMessage
	if err := conn.ReadJSON(&register); err != nil {
		log.Printf("control register read failed: %v", err)
		_ = conn.Close()
		return
	}
	if register.Type != "register" {
		_ = conn.WriteJSON(proto.ControlMessage{Type: "error", Error: "first control message must be register"})
		_ = conn.Close()
		return
	}

	sess, reversePort, err := s.registerSession(conn, register, cfg)
	if err != nil {
		_ = conn.WriteJSON(proto.ControlMessage{Type: "error", Error: err.Error()})
		_ = conn.Close()
		return
	}
	if reversePort > 0 {
		go s.listenReversePortOnce(reversePort)
	}

	ack := proto.ControlMessage{
		Type:                 "register_ack",
		SessionID:            sess.ID,
		HeartbeatIntervalSec: int(cfg.HeartbeatInterval.Seconds()),
		LeaseTTLSec:          int(cfg.LeaseTTL.Seconds()),
		ServerTime:           time.Now().UnixMilli(),
	}
	if reversePort > 0 {
		ack.ReverseTunnel = &proto.ReverseTunnel{Enabled: true, ListenPort: reversePort}
	}
	if err := conn.WriteJSON(ack); err != nil {
		s.disconnectSession(sess.ID, "register_ack_failed")
		_ = conn.Close()
		return
	}

	log.Printf("%s %s registered as session %s", sess.ComponentType, sess.AgentID, sess.ID)
	defer s.disconnectSession(sess.ID, "connection_closed")

	for {
		var msg proto.ControlMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if err != io.EOF {
				log.Printf("control read failed for %s: %v", sess.AgentID, err)
			}
			return
		}
		s.handleControlMessage(sess, msg, cfg)
	}
}

func (s *store) registerSession(conn *wslite.Conn, msg proto.ControlMessage, cfg config) (*session, int, error) {
	componentType := ""
	switch msg.Component {
	case proto.ComponentNexisNode:
		componentType = proto.AgentTypeNode
	case proto.ComponentNexisProbe:
		componentType = proto.AgentTypeProbe
	default:
		return nil, 0, fmt.Errorf("unknown component %q", msg.Component)
	}
	if msg.AgentID == "" || msg.Name == "" {
		return nil, 0, fmt.Errorf("agent_id and name are required")
	}
	if msg.Token == "" {
		return nil, 0, fmt.Errorf("token is required")
	}

	now := time.Now()
	sess := &session{
		ID:            newID("session"),
		AgentID:       msg.AgentID,
		ComponentType: componentType,
		Conn:          conn,
		ConnectedAt:   now,
		LastHeartbeat: now,
		LeaseExpires:  now.Add(cfg.LeaseTTL),
		IsActive:      true,
		RemoteAddr:    conn.RemoteAddr().String(),
	}

	var oldConn *wslite.Conn
	var oldSessionCopy *session
	var assignedReversePort int
	var persistItem *agent
	var persistReverseProbeID string
	var persistReversePort int

	s.mu.Lock()

	item := s.agents[msg.AgentID]
	if len(cfg.AgentTokenHashes) > 0 {
		expected := cfg.AgentTokenHashes[msg.AgentID]
		if expected == "" || expected != tokenHash(msg.Token) {
			s.mu.Unlock()
			return nil, 0, fmt.Errorf("token authentication failed")
		}
	}
	if item == nil {
		item = &agent{
			ID:            msg.AgentID,
			ComponentType: componentType,
			Name:          msg.Name,
			Status:        "online",
			TokenHash:     tokenHash(msg.Token),
			CreatedAt:     now,
			Meta:          map[string]any{},
		}
		s.agents[msg.AgentID] = item
	} else {
		if item.TokenHash != "" && item.TokenHash != tokenHash(msg.Token) {
			s.mu.Unlock()
			return nil, 0, fmt.Errorf("token authentication failed")
		}
		if item.SessionID != "" {
			old := s.sessions[item.SessionID]
			if old != nil && old.IsActive {
				old.IsActive = false
				old.CloseReason = "superseded"
				disconnected := now
				old.DisconnectedAt = &disconnected
				oldConn = old.Conn
				copyOld := *old
				oldSessionCopy = &copyOld
			}
		}
	}

	item.ComponentType = componentType
	item.Name = msg.Name
	item.Status = "online"
	item.LastSeenAt = &now
	item.UpdatedAt = now
	item.SessionID = sess.ID
	item.Meta = msg.Meta
	item.PublicIPv4 = stringFromMeta(msg.Meta, "public_ipv4", item.PublicIPv4)
	item.PublicIPv6 = stringFromMeta(msg.Meta, "public_ipv6", item.PublicIPv6)
	item.Region = stringFromMeta(msg.Meta, "region", item.Region)
	item.ISP = stringFromMeta(msg.Meta, "isp", item.ISP)
	item.NetworkStack = stringFromMeta(msg.Meta, "network_stack", item.NetworkStack)

	if componentType == proto.AgentTypeProbe && item.ReversePort == 0 {
		item.ReversePort = s.nextAvailableReversePortLocked(cfg)
		s.reversePortToProbe[item.ReversePort] = item.ID
	}
	if componentType == proto.AgentTypeProbe && item.ReversePort > 0 {
		s.reversePortToProbe[item.ReversePort] = item.ID
		assignedReversePort = item.ReversePort
	}
	s.sessions[sess.ID] = sess
	copyItem := *item
	persistItem = &copyItem
	if componentType == proto.AgentTypeProbe && item.ReversePort > 0 {
		persistReverseProbeID = item.ID
		persistReversePort = item.ReversePort
	}
	s.mu.Unlock()

	if oldConn != nil {
		go oldConn.Close()
	}
	s.persistSession(oldSessionCopy)
	s.persistAgent(persistItem)
	s.persistSession(sess)
	s.persistReversePort(persistReverseProbeID, persistReversePort)
	return sess, assignedReversePort, nil
}

func (s *store) nextAvailableReversePortLocked(cfg config) int {
	for port := s.nextReversePort; port <= cfg.ReverseEnd; port++ {
		if _, used := s.reversePortToProbe[port]; !used {
			s.nextReversePort = port + 1
			return port
		}
	}
	for port := cfg.ReverseStart; port < s.nextReversePort; port++ {
		if _, used := s.reversePortToProbe[port]; !used {
			s.nextReversePort = port + 1
			return port
		}
	}
	return 0
}

func (s *store) disconnectSession(sessionID, reason string) {
	now := time.Now()
	s.mu.Lock()
	sess := s.sessions[sessionID]
	if sess == nil || !sess.IsActive {
		s.mu.Unlock()
		return
	}
	sess.IsActive = false
	sess.CloseReason = reason
	sess.DisconnectedAt = &now
	if item := s.agents[sess.AgentID]; item != nil && item.SessionID == sessionID {
		item.SessionID = ""
		item.Status = "stale"
		item.UpdatedAt = now
	}
	sessCopy := *sess
	var agentCopy *agent
	if item := s.agents[sess.AgentID]; item != nil {
		copyItem := *item
		agentCopy = &copyItem
	}
	s.mu.Unlock()
	s.persistSession(&sessCopy)
	s.persistAgent(agentCopy)
}

func (s *store) handleControlMessage(sess *session, msg proto.ControlMessage, cfg config) {
	now := time.Now()
	switch msg.Type {
	case "heartbeat":
		var agentCopy *agent
		var sessionCopy *session
		s.mu.Lock()
		if item := s.agents[sess.AgentID]; item != nil {
			item.Status = "online"
			item.LastSeenAt = &now
			item.UpdatedAt = now
			if msg.Stats != nil {
				stats := *msg.Stats
				item.LatestStats = &stats
			}
			copyItem := *item
			agentCopy = &copyItem
		}
		if current := s.sessions[sess.ID]; current != nil {
			current.LastHeartbeat = now
			current.LeaseExpires = now.Add(cfg.LeaseTTL)
			copySession := *current
			sessionCopy = &copySession
		}
		s.mu.Unlock()
		s.persistAgent(agentCopy)
		s.persistSession(sessionCopy)
		if sess.ComponentType == proto.AgentTypeNode && msg.Stats != nil {
			s.addSnapshot(sess.AgentID, *msg.Stats, now)
		}
	case "test_result":
		if msg.Result == nil {
			return
		}
		s.addResult(tcpResult{
			TS:        now,
			NodeID:    msg.NodeID,
			ProbeID:   msg.ProbeID,
			TestType:  msg.TestType,
			Direction: msg.Direction,
			PathMode:  msg.PathMode,
			Result:    *msg.Result,
			Raw:       msg.Raw,
		})
	case "reverse_stream_data":
		s.writeReverseStream(msg.StreamID, msg.Data)
	case "reverse_stream_close":
		s.closeReverseStream(msg.StreamID)
	}
}

func (s *store) schedulerLoop(ctx context.Context, cfg config) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.dispatchScheduledTests(cfg)
			timer.Reset(cfg.ScheduleInterval)
		}
	}
}

func (s *store) dispatchScheduledTests(cfg config) {
	nodes := s.onlineAgents(proto.AgentTypeNode)
	probes := s.onlineAgents(proto.AgentTypeProbe)
	if len(nodes) == 0 || len(probes) == 0 {
		return
	}
	params := proto.DefaultTestParams()
	for _, node := range nodes {
		host := node.PublicIPv4
		if host == "" {
			host = stringFromMeta(node.Meta, "test_host", "127.0.0.1")
		}
		port := intFromMeta(node.Meta, "listener_port", 47211)
		for _, probe := range probes {
			forward := proto.ControlMessage{
				Type:     "run_test",
				JobID:    newID("job_forward"),
				TestType: proto.TestForwardDirectTCP,
				NodeID:   node.ID,
				ProbeID:  probe.ID,
				Target:   &proto.TestTarget{Host: host, Port: port},
				Params:   &params,
			}
			if err := s.sendToAgent(probe.ID, forward); err != nil {
				log.Printf("dispatch forward test failed: %v", err)
			}
			if probe.ReversePort > 0 {
				reverse := proto.ControlMessage{
					Type:     "run_test",
					JobID:    newID("job_reverse"),
					TestType: proto.TestReverseLogicalTCP,
					NodeID:   node.ID,
					ProbeID:  probe.ID,
					Target:   &proto.TestTarget{Host: cfg.PublicHost, Port: probe.ReversePort},
					Params:   &params,
				}
				if err := s.sendToAgent(node.ID, reverse); err != nil {
					log.Printf("dispatch reverse test failed: %v", err)
				}
			}
		}
	}
}

func (s *store) sendToAgent(agentID string, msg proto.ControlMessage) error {
	sess := s.sessionByAgent(agentID)
	if sess == nil {
		return fmt.Errorf("agent %s has no active session", agentID)
	}
	return sess.Conn.WriteJSON(msg)
}

func (s *store) listenReversePortOnce(port int) {
	s.mu.Lock()
	if s.reverseListeners[port] {
		s.mu.Unlock()
		return
	}
	s.reverseListeners[port] = true
	s.mu.Unlock()
	s.listenReversePort(port)
}

func (s *store) listenReversePort(port int) {
	if port == 0 {
		return
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		s.mu.Lock()
		delete(s.reverseListeners, port)
		s.mu.Unlock()
		log.Printf("reverse port %d listen failed: %v", port, err)
		return
	}
	defer func() {
		s.mu.Lock()
		delete(s.reverseListeners, port)
		s.mu.Unlock()
	}()
	log.Printf("reverse logical TCP listening on :%d", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("reverse accept on %d failed: %v", port, err)
			return
		}
		go s.handleReverseTCP(port, conn)
	}
}

func (s *store) handleReverseTCP(port int, conn net.Conn) {
	s.mu.RLock()
	probeID := s.reversePortToProbe[port]
	s.mu.RUnlock()
	if probeID == "" {
		_ = conn.Close()
		return
	}
	sess := s.sessionByAgent(probeID)
	if sess == nil {
		_ = conn.Close()
		return
	}

	streamID := newID("stream")
	stream := &reverseStream{ID: streamID, ProbeID: probeID, Conn: conn, CreatedAt: time.Now()}
	s.mu.Lock()
	s.streams[streamID] = stream
	s.mu.Unlock()
	defer s.closeReverseStream(streamID)

	if err := sess.Conn.WriteJSON(proto.ControlMessage{Type: "reverse_stream_open", StreamID: streamID}); err != nil {
		return
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			data := base64.StdEncoding.EncodeToString(buf[:n])
			if err := sess.Conn.WriteJSON(proto.ControlMessage{Type: "reverse_stream_data", StreamID: streamID, Data: data}); err != nil {
				return
			}
		}
		if err != nil {
			_ = sess.Conn.WriteJSON(proto.ControlMessage{Type: "reverse_stream_close", StreamID: streamID, Reason: err.Error()})
			return
		}
	}
}

func (s *store) writeReverseStream(streamID, encoded string) {
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return
	}
	s.mu.RLock()
	stream := s.streams[streamID]
	s.mu.RUnlock()
	if stream == nil {
		return
	}
	_, _ = stream.Conn.Write(payload)
}

func (s *store) closeReverseStream(streamID string) {
	s.mu.Lock()
	stream := s.streams[streamID]
	delete(s.streams, streamID)
	s.mu.Unlock()
	if stream != nil {
		_ = stream.Conn.Close()
	}
}

func stringFromMeta(meta map[string]any, key string, fallback string) string {
	if meta == nil {
		return fallback
	}
	if value, ok := meta[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func intFromMeta(meta map[string]any, key string, fallback int) int {
	if meta == nil {
		return fallback
	}
	switch value := meta[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
