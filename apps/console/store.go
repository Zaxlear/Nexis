package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"time"

	"nexis/packages/proto"
)

func (s *store) agentViews(componentType string) []proto.AgentView {
	s.mu.RLock()
	defer s.mu.RUnlock()

	views := make([]proto.AgentView, 0, len(s.agents))
	for _, item := range s.agents {
		if componentType != "" && item.ComponentType != componentType {
			continue
		}
		views = append(views, s.agentViewLocked(item))
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Status == views[j].Status {
			return views[i].Name < views[j].Name
		}
		return views[i].Status == "online"
	})
	return views
}

func (s *store) agentView(id string) (proto.AgentView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.agents[id]
	if !ok {
		return proto.AgentView{}, false
	}
	return s.agentViewLocked(item), true
}

func (s *store) agentViewLocked(item *agent) proto.AgentView {
	view := proto.AgentView{
		ID:            item.ID,
		ComponentType: item.ComponentType,
		Name:          item.Name,
		Region:        item.Region,
		ISP:           item.ISP,
		NetworkStack:  item.NetworkStack,
		PublicIPv4:    item.PublicIPv4,
		PublicIPv6:    item.PublicIPv6,
		Status:        item.Status,
		LastSeenAt:    item.LastSeenAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		ReversePort:   item.ReversePort,
		Meta:          item.Meta,
		LatestStats:   item.LatestStats,
		Summary:       map[string]any{},
	}
	if item.ComponentType == proto.AgentTypeNode {
		view.Summary = s.nodeSummaryLocked(item.ID)
	}
	return view
}

func (s *store) nodeSummaryLocked(nodeID string) map[string]any {
	var forward []float64
	var reverse []float64
	for _, result := range s.results {
		if result.NodeID != nodeID {
			continue
		}
		if result.Direction == proto.DirectionForward && result.Result.AppRTTMS > 0 {
			forward = append(forward, result.Result.AppRTTMS)
		}
		if result.Direction == proto.DirectionReverse && result.Result.AppRTTMS > 0 {
			reverse = append(reverse, result.Result.AppRTTMS)
		}
	}
	return map[string]any{
		"forward_rtt_ms": round(avgLast(forward, 8), 1),
		"reverse_rtt_ms": round(avgLast(reverse, 8), 1),
	}
}

func (s *store) addSnapshot(nodeID string, stats proto.SystemStats, ts time.Time) {
	s.mu.Lock()
	s.nextSnapshotID++
	snapshot := systemSnapshot{ID: s.nextSnapshotID, NodeID: nodeID, TS: ts, Stats: stats}
	s.snapshots = append(s.snapshots, snapshot)
	var changedAgent *agent
	if item := s.agents[nodeID]; item != nil {
		copyStats := stats
		item.LatestStats = &copyStats
		item.UpdatedAt = ts
		agentCopy := *item
		changedAgent = &agentCopy
	}
	if len(s.snapshots) > 10000 {
		s.snapshots = append([]systemSnapshot(nil), s.snapshots[len(s.snapshots)-9000:]...)
	}
	s.mu.Unlock()
	s.persistSnapshot(snapshot)
	s.persistAgent(changedAgent)
}

func (s *store) addResult(result tcpResult) {
	s.mu.Lock()
	s.nextResultID++
	result.ID = s.nextResultID
	if result.TS.IsZero() {
		result.TS = time.Now()
	}
	s.results = append(s.results, result)
	if len(s.results) > 30000 {
		s.results = append([]tcpResult(nil), s.results[len(s.results)-27000:]...)
	}
	s.mu.Unlock()
	s.persistResult(result)
}

func (s *store) resultsFor(nodeID string, since time.Time) []tcpResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []tcpResult
	for _, item := range s.results {
		if item.NodeID == nodeID && !item.TS.Before(since) {
			out = append(out, item)
		}
	}
	return out
}

func (s *store) snapshotsFor(nodeID string, since time.Time) []systemSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []systemSnapshot
	for _, item := range s.snapshots {
		if item.NodeID == nodeID && !item.TS.Before(since) {
			out = append(out, item)
		}
	}
	return out
}

func (s *store) onlineAgents(componentType string) []*agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*agent
	for _, item := range s.agents {
		if item.ComponentType == componentType && item.Status == "online" && item.SessionID != "" {
			copyItem := *item
			out = append(out, &copyItem)
		}
	}
	return out
}

func (s *store) sessionByAgent(agentID string) *session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item := s.agents[agentID]
	if item == nil || item.SessionID == "" {
		return nil
	}
	sess := s.sessions[item.SessionID]
	if sess == nil || !sess.IsActive {
		return nil
	}
	return sess
}

func (s *store) expireLoop(done <-chan struct{}, cfg config) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			now := time.Now()
			var changed []*agent
			s.mu.Lock()
			for _, item := range s.agents {
				if item.Demo {
					continue
				}
				oldStatus := item.Status
				if item.LastSeenAt == nil {
					item.Status = "offline"
				} else {
					age := now.Sub(*item.LastSeenAt)
					switch {
					case age > cfg.LeaseTTL:
						item.Status = "offline"
						item.SessionID = ""
					case age > cfg.HeartbeatTimeout:
						item.Status = "stale"
					default:
						item.Status = "online"
					}
				}
				if item.Status != oldStatus {
					item.UpdatedAt = now
					copyItem := *item
					changed = append(changed, &copyItem)
				}
			}
			s.mu.Unlock()
			for _, item := range changed {
				s.persistAgent(item)
			}
		}
	}
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func avgLast(values []float64, limit int) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) > limit {
		values = values[len(values)-limit:]
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
