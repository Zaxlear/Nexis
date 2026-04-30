package main

import (
	"fmt"
	"net/http"
	"time"

	"nexis/packages/proto"
)

func debugHandler(st *store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		snapshot := st.debugSnapshot()
		fmt.Fprintf(w, "nexis_agents_total %d\n", snapshot.AgentCount)
		fmt.Fprintf(w, "nexis_nodes_online %d\n", snapshot.OnlineNodes)
		fmt.Fprintf(w, "nexis_probes_online %d\n", snapshot.OnlineProbes)
		fmt.Fprintf(w, "nexis_sessions_active %d\n", snapshot.ActiveSessions)
		fmt.Fprintf(w, "nexis_tcp_results_total %d\n", snapshot.ResultCount)
		fmt.Fprintf(w, "nexis_system_snapshots_total %d\n", snapshot.SnapshotCount)
		fmt.Fprintf(w, "nexis_reverse_streams_active %d\n", snapshot.ActiveStreams)
		fmt.Fprintf(w, "nexis_database_enabled %d\n", boolMetric(snapshot.DatabaseMode == "postgres"))
	})
	mux.HandleFunc("/debug/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, st.debugSnapshot())
	})
	return mux
}

type debugSnapshot struct {
	TS             time.Time `json:"ts"`
	AgentCount     int       `json:"agent_count"`
	OnlineNodes    int       `json:"online_nodes"`
	OnlineProbes   int       `json:"online_probes"`
	ActiveSessions int       `json:"active_sessions"`
	ResultCount    int       `json:"result_count"`
	SnapshotCount  int       `json:"snapshot_count"`
	ActiveStreams  int       `json:"active_streams"`
	DatabaseMode   string    `json:"database_mode"`
}

func (s *store) debugSnapshot() debugSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := debugSnapshot{
		TS:            time.Now(),
		AgentCount:    len(s.agents),
		ResultCount:   len(s.results),
		SnapshotCount: len(s.snapshots),
		ActiveStreams: len(s.streams),
		DatabaseMode:  s.databaseMode(),
	}
	for _, item := range s.agents {
		if item.ComponentType == proto.AgentTypeNode && item.Status == "online" {
			out.OnlineNodes++
		}
		if item.ComponentType == proto.AgentTypeProbe && item.Status == "online" {
			out.OnlineProbes++
		}
	}
	for _, sess := range s.sessions {
		if sess.IsActive {
			out.ActiveSessions++
		}
	}
	return out
}

func boolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
}
