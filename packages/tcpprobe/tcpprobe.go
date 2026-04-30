package tcpprobe

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"time"

	"nexis/packages/proto"
)

type ProbeFrame struct {
	Type    string `json:"type"`
	TaskID  string `json:"task_id"`
	Seq     int    `json:"seq"`
	TS      int64  `json:"ts"`
	Payload string `json:"payload"`
}

func Run(ctx context.Context, target proto.TestTarget, params proto.TestParams, taskID string) (proto.TestResult, map[string]any, error) {
	params = normalizeParams(params)
	address := fmt.Sprintf("%s:%d", target.Host, target.Port)
	dialer := net.Dialer{Timeout: time.Duration(params.ConnectTimeoutMS) * time.Millisecond}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", address)
	connectLatency := float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		return proto.TestResult{
			ConnectLatencyMS: connectLatency,
			SuccessRate:      0,
			TimeoutRate:      1,
			ProbeLossRate:    1,
			SampleCount:      params.ProbeCount,
		}, map[string]any{"samples": []float64{}, "error": err.Error()}, err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	var samples []float64
	var timeouts int
	var failures int

	for seq := 1; seq <= params.ProbeCount; seq++ {
		select {
		case <-ctx.Done():
			return aggregate(connectLatency, samples, params.ProbeCount, timeouts, failures), raw(samples, ctx.Err()), ctx.Err()
		default:
		}

		frame := ProbeFrame{
			Type:    "probe",
			TaskID:  taskID,
			Seq:     seq,
			TS:      time.Now().UnixMilli(),
			Payload: "ping",
		}
		payload, _ := json.Marshal(frame)

		if err := conn.SetWriteDeadline(time.Now().Add(time.Duration(params.WriteTimeoutMS) * time.Millisecond)); err != nil {
			failures++
			continue
		}
		rttStart := time.Now()
		if _, err := writer.Write(append(payload, '\n')); err != nil {
			failures++
			continue
		}
		if err := writer.Flush(); err != nil {
			failures++
			continue
		}

		if err := conn.SetReadDeadline(time.Now().Add(time.Duration(params.ReadTimeoutMS) * time.Millisecond)); err != nil {
			failures++
			continue
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if isTimeout(err) {
				timeouts++
			}
			failures++
			continue
		}
		var ack ProbeFrame
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &ack); err != nil {
			failures++
			continue
		}
		if ack.Type != "probe_ack" || ack.TaskID != taskID || ack.Seq != seq {
			failures++
			continue
		}
		samples = append(samples, float64(time.Since(rttStart).Microseconds())/1000)

		if seq < params.ProbeCount && params.IntervalMS > 0 {
			timer := time.NewTimer(time.Duration(params.IntervalMS) * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return aggregate(connectLatency, samples, params.ProbeCount, timeouts, failures), raw(samples, ctx.Err()), ctx.Err()
			case <-timer.C:
			}
		}
	}

	return aggregate(connectLatency, samples, params.ProbeCount, timeouts, failures), raw(samples, nil), nil
}

func HandleEchoConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(io.LimitReader(conn, 1<<20))
	writer := bufio.NewWriter(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		ack, ok := AckForLine([]byte(line))
		if !ok {
			return
		}
		conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if _, err := writer.Write(append(ack, '\n')); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
	}
}

func AckForLine(line []byte) ([]byte, bool) {
	var frame ProbeFrame
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(line))), &frame); err != nil {
		return nil, false
	}
	if frame.Type != "probe" {
		return nil, false
	}
	ack := ProbeFrame{
		Type:    "probe_ack",
		TaskID:  frame.TaskID,
		Seq:     frame.Seq,
		TS:      time.Now().UnixMilli(),
		Payload: "pong",
	}
	out, _ := json.Marshal(ack)
	return out, true
}

func normalizeParams(params proto.TestParams) proto.TestParams {
	def := proto.DefaultTestParams()
	if params.ProbeCount <= 0 {
		params.ProbeCount = def.ProbeCount
	}
	if params.ConnectTimeoutMS <= 0 {
		params.ConnectTimeoutMS = def.ConnectTimeoutMS
	}
	if params.ReadTimeoutMS <= 0 {
		params.ReadTimeoutMS = def.ReadTimeoutMS
	}
	if params.WriteTimeoutMS <= 0 {
		params.WriteTimeoutMS = def.WriteTimeoutMS
	}
	if params.IntervalMS <= 0 {
		params.IntervalMS = def.IntervalMS
	}
	return params
}

func aggregate(connectLatency float64, samples []float64, expected int, timeouts int, failures int) proto.TestResult {
	success := len(samples)
	if expected <= 0 {
		expected = 1
	}
	return proto.TestResult{
		ConnectLatencyMS: connectLatency,
		AppRTTMS:         average(samples),
		JitterMS:         jitter(samples),
		SuccessRate:      float64(success) / float64(expected),
		TimeoutRate:      float64(timeouts) / float64(expected),
		ProbeLossRate:    float64(expected-success) / float64(expected),
		SampleCount:      expected,
	}
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func jitter(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(values); i++ {
		total += math.Abs(values[i] - values[i-1])
	}
	return total / float64(len(values)-1)
}

func raw(samples []float64, err error) map[string]any {
	out := map[string]any{"samples": samples}
	if err != nil {
		out["error"] = err.Error()
	}
	return out
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
