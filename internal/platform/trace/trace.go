package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jinguo998/agent-sessions/internal/app/ports"
)

const EnvVar = "AGENT_SESSIONS_TRACE"

// FileTracer writes structured JSONL trace events to a local file.
type FileTracer struct {
	path string
	mu   sync.Mutex
}

var _ ports.Tracer = (*FileTracer)(nil)

// NewFromEnv returns a FileTracer configured from AGENT_SESSIONS_TRACE.
// When the environment variable is unset, the returned tracer is disabled.
func NewFromEnv() *FileTracer {
	return NewFile(os.Getenv(EnvVar))
}

// NewFile returns a FileTracer targeting path. An empty path yields a no-op tracer.
func NewFile(path string) *FileTracer {
	return &FileTracer{path: path}
}

func (t *FileTracer) Enabled() bool {
	return t != nil && t.path != ""
}

func (t *FileTracer) Event(event string, fields map[string]any) {
	if !t.Enabled() {
		return
	}

	payload := make(map[string]any, len(fields)+3)
	payload["ts"] = time.Now().Format(time.RFC3339Nano)
	payload["event"] = event
	payload["pid"] = os.Getpid()
	for k, v := range fields {
		payload[k] = v
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	data = append(data, '\n')

	t.mu.Lock()
	defer t.mu.Unlock()

	if dir := filepath.Dir(t.path); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(t.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}
