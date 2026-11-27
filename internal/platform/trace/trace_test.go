package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTracerDisabledWhenPathEmpty(t *testing.T) {
	tracer := NewFile("")
	if tracer.Enabled() {
		t.Fatal("tracer with empty path should be disabled")
	}
	tracer.Event("noop", map[string]any{"k": "v"})
}

func TestFileTracerAppendsJSONLEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "trace.jsonl")
	tracer := NewFile(path)
	if !tracer.Enabled() {
		t.Fatal("tracer with path should be enabled")
	}

	tracer.Event("scan_start", map[string]any{"token": 7})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("trace lines = %d, want 1", len(lines))
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got["event"] != "scan_start" {
		t.Fatalf("event = %v, want scan_start", got["event"])
	}
	if got["token"] != float64(7) {
		t.Fatalf("token = %v, want 7", got["token"])
	}
	if _, ok := got["ts"]; !ok {
		t.Fatal("missing ts field")
	}
}
