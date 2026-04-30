package tcpprobe

import (
	"encoding/json"
	"testing"
)

func TestAckForLine(t *testing.T) {
	ack, ok := AckForLine([]byte(`{"type":"probe","task_id":"t1","seq":2,"ts":1,"payload":"ping"}` + "\n"))
	if !ok {
		t.Fatal("expected ack")
	}
	var frame ProbeFrame
	if err := json.Unmarshal(ack, &frame); err != nil {
		t.Fatalf("ack is not json: %v", err)
	}
	if frame.Type != "probe_ack" || frame.TaskID != "t1" || frame.Seq != 2 || frame.Payload != "pong" {
		t.Fatalf("unexpected ack: %+v", frame)
	}
}

func TestJitter(t *testing.T) {
	got := jitter([]float64{10, 15, 12})
	if got != 4 {
		t.Fatalf("expected jitter 4, got %v", got)
	}
}
