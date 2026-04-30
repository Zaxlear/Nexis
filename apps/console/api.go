package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"nexis/packages/proto"
)

func apiHandler(st *store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		withAPI(w, r, func() {
			writeJSON(w, st.agentViews(proto.AgentTypeNode))
		})
	})
	mux.HandleFunc("/api/v1/nodes/", func(w http.ResponseWriter, r *http.Request) {
		withAPI(w, r, func() {
			handleNodeAPI(st, w, r)
		})
	})
	mux.HandleFunc("/api/v1/probes", func(w http.ResponseWriter, r *http.Request) {
		withAPI(w, r, func() {
			writeJSON(w, st.agentViews(proto.AgentTypeProbe))
		})
	})
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		withAPI(w, r, func() {
			writeJSON(w, map[string]any{"status": "ok", "time": time.Now()})
		})
	})
	return mux
}

func withAPI(w http.ResponseWriter, r *http.Request, next func()) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	next()
}

func handleNodeAPI(st *store, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "nodes" {
		http.NotFound(w, r)
		return
	}
	nodeID := parts[3]
	if len(parts) == 4 {
		view, ok := st.agentView(nodeID)
		if !ok || view.ComponentType != proto.AgentTypeNode {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, view)
		return
	}

	switch parts[4] {
	case "resource-series":
		writeJSON(w, resourceSeries(st, nodeID, r.URL.Query().Get("range"), r.URL.Query().Get("metric")))
	case "connectivity":
		writeJSON(w, connectivity(st, nodeID, r.URL.Query().Get("range")))
	case "probes":
		if len(parts) == 7 && parts[6] == "series" {
			probeID := parts[5]
			writeJSON(w, probeSeries(st, nodeID, probeID, r.URL.Query().Get("range"), r.URL.Query().Get("metric"), r.URL.Query().Get("direction")))
			return
		}
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

func resourceSeries(st *store, nodeID, rangeName, metric string) map[string]any {
	if metric == "" {
		metric = "cpu"
	}
	since := time.Now().Add(-rangeDuration(rangeName))
	snapshots := st.snapshotsFor(nodeID, since)
	points := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		value := resourceMetric(snapshot.Stats, metric)
		points = append(points, map[string]any{"ts": snapshot.TS, "value": value})
	}
	return map[string]any{"metric": metric, "range": normalizeRange(rangeName), "points": points}
}

func connectivity(st *store, nodeID, rangeName string) map[string]any {
	since := time.Now().Add(-rangeDuration(rangeName))
	results := st.resultsFor(nodeID, since)
	probes := st.agentViews(proto.AgentTypeProbe)

	type latest struct {
		forward *tcpResult
		reverse *tcpResult
	}
	byProbe := make(map[string]*latest)
	for _, probe := range probes {
		byProbe[probe.ID] = &latest{}
	}
	for i := range results {
		item := results[i]
		entry := byProbe[item.ProbeID]
		if entry == nil {
			entry = &latest{}
			byProbe[item.ProbeID] = entry
		}
		if item.Direction == proto.DirectionForward && (entry.forward == nil || item.TS.After(entry.forward.TS)) {
			copyItem := item
			entry.forward = &copyItem
		}
		if item.Direction == proto.DirectionReverse && (entry.reverse == nil || item.TS.After(entry.reverse.TS)) {
			copyItem := item
			entry.reverse = &copyItem
		}
	}

	items := make([]map[string]any, 0, len(probes))
	for _, probe := range probes {
		entry := byProbe[probe.ID]
		var forward *proto.TestResult
		var reverse *proto.TestResult
		var last *time.Time
		if entry != nil && entry.forward != nil {
			copyResult := entry.forward.Result
			forward = &copyResult
			ts := entry.forward.TS
			last = &ts
		}
		if entry != nil && entry.reverse != nil {
			copyResult := entry.reverse.Result
			reverse = &copyResult
			if last == nil || entry.reverse.TS.After(*last) {
				ts := entry.reverse.TS
				last = &ts
			}
		}
		items = append(items, map[string]any{
			"probe":        probe,
			"forward":      forward,
			"reverse":      reverse,
			"health":       linkHealth(forward, reverse),
			"last_test_at": last,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		leftTime, leftHas := itemLastTestTime(items[i])
		rightTime, rightHas := itemLastTestTime(items[j])
		if leftHas != rightHas {
			return leftHas
		}
		if leftHas && rightHas && !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
		leftProbe := items[i]["probe"].(proto.AgentView)
		rightProbe := items[j]["probe"].(proto.AgentView)
		return leftProbe.Name < rightProbe.Name
	})
	return map[string]any{"range": normalizeRange(rangeName), "items": items}
}

func itemLastTestTime(item map[string]any) (time.Time, bool) {
	value, ok := item["last_test_at"].(*time.Time)
	if !ok || value == nil {
		return time.Time{}, false
	}
	return *value, true
}

func probeSeries(st *store, nodeID, probeID, rangeName, metric, direction string) map[string]any {
	if metric == "" {
		metric = "app_rtt"
	}
	if direction == "" {
		direction = proto.DirectionForward
	}
	since := time.Now().Add(-rangeDuration(rangeName))
	results := st.resultsFor(nodeID, since)
	points := make([]map[string]any, 0, len(results))
	for _, item := range results {
		if item.ProbeID != probeID || item.Direction != direction {
			continue
		}
		points = append(points, map[string]any{"ts": item.TS, "value": resultMetric(item.Result, metric)})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i]["ts"].(time.Time).Before(points[j]["ts"].(time.Time))
	})
	return map[string]any{
		"node_id":   nodeID,
		"probe_id":  probeID,
		"range":     normalizeRange(rangeName),
		"metric":    metric,
		"direction": direction,
		"points":    points,
	}
}

func resourceMetric(stats proto.SystemStats, metric string) float64 {
	switch metric {
	case "memory":
		if stats.MemoryTotalMB <= 0 {
			return 0
		}
		return round(stats.MemoryUsedMB/stats.MemoryTotalMB*100, 1)
	case "network":
		return round(float64(stats.NetworkRXBytes+stats.NetworkTXBytes)/1024/1024, 1)
	case "disk":
		if stats.DiskTotalGB <= 0 {
			return 0
		}
		return round(stats.DiskUsedGB/stats.DiskTotalGB*100, 1)
	default:
		return round(stats.CPUUsagePercent, 1)
	}
}

func resultMetric(result proto.TestResult, metric string) float64 {
	switch metric {
	case "connect_latency":
		return round(result.ConnectLatencyMS, 1)
	case "jitter":
		return round(result.JitterMS, 1)
	case "success_rate":
		return round(result.SuccessRate*100, 1)
	case "timeout_rate":
		return round(result.TimeoutRate*100, 1)
	case "probe_loss_rate":
		return round(result.ProbeLossRate*100, 1)
	default:
		return round(result.AppRTTMS, 1)
	}
}

func linkHealth(forward *proto.TestResult, reverse *proto.TestResult) string {
	worst := "unknown"
	for _, result := range []*proto.TestResult{forward, reverse} {
		if result == nil {
			continue
		}
		current := "healthy"
		if result.SuccessRate < 0.8 || result.TimeoutRate >= 0.2 {
			current = "critical"
		} else if result.SuccessRate < 0.95 || result.AppRTTMS >= 150 || result.JitterMS >= 80 {
			current = "degraded"
		}
		if severity(current) > severity(worst) {
			worst = current
		}
	}
	return worst
}

func severity(status string) int {
	switch status {
	case "critical":
		return 3
	case "degraded":
		return 2
	case "healthy":
		return 1
	default:
		return 0
	}
}

func rangeDuration(value string) time.Duration {
	switch normalizeRange(value) {
	case "6h":
		return 6 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}

func normalizeRange(value string) string {
	switch value {
	case "6h", "24h", "7d":
		return value
	default:
		return "1h"
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}
