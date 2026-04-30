package configlite

import "testing"

func TestPathFromArgs(t *testing.T) {
	if got := PathFromArgs([]string{"-config", "x.yaml"}, "config"); got != "x.yaml" {
		t.Fatalf("expected x.yaml, got %q", got)
	}
	if got := PathFromArgs([]string{"--config=y.yaml"}, "config"); got != "y.yaml" {
		t.Fatalf("expected y.yaml, got %q", got)
	}
}

func TestValues(t *testing.T) {
	values := Values{"console.web_port": "47131", "console.seed_demo": "true"}
	if got := values.Int("console.web_port", 1); got != 47131 {
		t.Fatalf("expected 47131, got %d", got)
	}
	if !values.Bool("console.seed_demo", false) {
		t.Fatal("expected true")
	}
	if got := values.String("missing", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}
