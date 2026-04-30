package main

import (
	"math"
	"math/rand"
	"time"

	"nexis/packages/proto"
)

func (s *store) seedDemoData() {
	now := time.Now()
	node := &agent{
		ID:            "demo-node-oracle",
		ComponentType: proto.AgentTypeNode,
		Name:          "Oracle",
		PublicIPv4:    "140.245.44.107",
		Status:        "online",
		TokenHash:     tokenHash("demo"),
		LastSeenAt:    &now,
		CreatedAt:     now.Add(-41*24*time.Hour - 8*time.Hour - 19*time.Minute),
		UpdatedAt:     now,
		Meta: map[string]any{
			"os":             "linux",
			"arch":           "arm64",
			"virtualization": "guest",
			"listener_port":  47211,
			"hardware":       "Neoverse-N1",
		},
		Demo: true,
	}
	stats := proto.SystemStats{
		CPUUsagePercent: 12.5,
		CPUCores:        4,
		MemoryTotalMB:   23961,
		MemoryUsedMB:    8243,
		DiskTotalGB:     195.8,
		DiskUsedGB:      61.3,
		NetworkRXBytes:  8_940_000_000,
		NetworkTXBytes:  4_210_000_000,
		UptimeSeconds:   int64(now.Sub(node.CreatedAt).Seconds()),
		OS:              "linux",
		Arch:            "arm64",
		Virtualization:  "guest",
	}
	node.LatestStats = &stats

	probes := []*agent{
		demoProbe("demo-probe-ct-v4", "广东电信IPv4", "广东", "电信", "IPv4", now),
		demoProbe("demo-probe-ct-v6", "广东电信IPv6", "广东", "电信", "IPv6", now),
		demoProbe("demo-probe-cm-v4", "广东移动IPv4", "广东", "移动", "IPv4", now),
		demoProbe("demo-probe-cm-v6", "广东移动IPv6", "广东", "移动", "IPv6", now),
		demoProbe("demo-probe-cu-v4", "广东联通IPv4", "广东", "联通", "IPv4", now),
		demoProbe("demo-probe-cu-v6", "广东联通IPv6", "广东", "联通", "IPv6", now),
	}

	s.mu.Lock()
	s.agents[node.ID] = node
	for _, probe := range probes {
		s.agents[probe.ID] = probe
	}
	s.mu.Unlock()
	s.persistAgent(node)
	for _, probe := range probes {
		s.persistAgent(probe)
	}

	rng := rand.New(rand.NewSource(42))
	start := now.Add(-7 * 24 * time.Hour)
	step := 15 * time.Minute
	for ts := start; ts.Before(now); ts = ts.Add(step) {
		wave := math.Sin(float64(ts.Unix()) / 18000)
		stats := proto.SystemStats{
			CPUUsagePercent: 11 + wave*4 + rng.Float64()*3,
			CPUCores:        4,
			MemoryTotalMB:   23961,
			MemoryUsedMB:    8200 + wave*360 + rng.Float64()*240,
			DiskTotalGB:     195.8,
			DiskUsedGB:      61.3 + rng.Float64()*0.8,
			NetworkRXBytes:  8_940_000_000 + int64(ts.Sub(start).Minutes())*95000,
			NetworkTXBytes:  4_210_000_000 + int64(ts.Sub(start).Minutes())*61000,
			UptimeSeconds:   int64(ts.Sub(node.CreatedAt).Seconds()),
			OS:              "linux",
			Arch:            "arm64",
			Virtualization:  "guest",
		}
		s.addSnapshot(node.ID, stats, ts)
		for index, probe := range probes {
			base := []float64{42, 48, 58, 64, 96, 118}[index]
			spike := 0.0
			if rng.Float64() < 0.035 {
				spike = 300 + rng.Float64()*850
			}
			forwardRTT := base + rng.Float64()*12 + spike
			reverseRTT := base*1.18 + rng.Float64()*18 + spike*0.42
			forwardLoss := 0.0
			reverseLoss := 0.0
			if rng.Float64() < 0.04 {
				forwardLoss = []float64{0.1, 0.2, 0.4}[rng.Intn(3)]
			}
			if rng.Float64() < 0.05 {
				reverseLoss = []float64{0.1, 0.2, 0.5}[rng.Intn(3)]
			}
			s.addResult(tcpResult{
				TS:        ts,
				NodeID:    node.ID,
				ProbeID:   probe.ID,
				TestType:  proto.TestForwardDirectTCP,
				Direction: proto.DirectionForward,
				PathMode:  proto.PathModeDirect,
				Result: proto.TestResult{
					ConnectLatencyMS: forwardRTT * 0.72,
					AppRTTMS:         forwardRTT,
					JitterMS:         2 + rng.Float64()*16 + spike*0.03,
					SuccessRate:      1 - forwardLoss,
					TimeoutRate:      forwardLoss / 2,
					ProbeLossRate:    forwardLoss,
					SampleCount:      5,
				},
				Raw: map[string]any{"demo": true},
			})
			s.addResult(tcpResult{
				TS:        ts,
				NodeID:    node.ID,
				ProbeID:   probe.ID,
				TestType:  proto.TestReverseLogicalTCP,
				Direction: proto.DirectionReverse,
				PathMode:  proto.PathModeTunnel,
				Result: proto.TestResult{
					ConnectLatencyMS: reverseRTT * 0.67,
					AppRTTMS:         reverseRTT,
					JitterMS:         3 + rng.Float64()*18 + spike*0.02,
					SuccessRate:      1 - reverseLoss,
					TimeoutRate:      reverseLoss / 2,
					ProbeLossRate:    reverseLoss,
					SampleCount:      5,
				},
				Raw: map[string]any{"demo": true},
			})
		}
	}
}

func demoProbe(id, name, region, isp, stack string, now time.Time) *agent {
	return &agent{
		ID:            id,
		ComponentType: proto.AgentTypeProbe,
		Name:          name,
		Region:        region,
		ISP:           isp,
		NetworkStack:  stack,
		Status:        "online",
		TokenHash:     tokenHash("demo"),
		LastSeenAt:    &now,
		CreatedAt:     now.Add(-18 * time.Hour),
		UpdatedAt:     now,
		Meta:          map[string]any{"os": "linux", "arch": "amd64"},
		Demo:          true,
	}
}
