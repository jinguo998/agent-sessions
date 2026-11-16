package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernTranscript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	path := filepath.Join(root, "projects", "-tmp-demo", "session.jsonl")
	child := filepath.Join(strings.TrimSuffix(path, ".jsonl"), "subagents", "agent-sample")
	if err := os.MkdirAll(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		path: `{"type":"ai-title","aiTitle":"Generated title"}
{"type":"custom-title","customTitle":"Review session"}
{"type":"user","message":{"content":[{"type":"text","text":"Inspect this screenshot"},{"type":"image","source":{"type":"base64","data":"TEST"}}]}}
{"type":"assistant","message":{"model":"claude-sonnet","content":[{"type":"tool_use","id":"tool-agent","name":"Agent","input":{"description":"Review"}}]}}
{"type":"user","isSidechain":true,"agentId":"background","message":{"content":"Internal subagent prompt"}}
{"type":"assistant","isSidechain":true,"agentId":"background","message":{"model":"claude-haiku","content":[{"type":"text","text":"Embedded agent response"}]}}
{"type":"attachment","attachment":{"type":"goal_status","condition":"Review done","met":true,"reason":"All checks passed"}}
`,
		child + ".meta.json": `{"toolUseId":"tool-agent","agentType":"Explore","description":"Inspect sample"}`,
		child + ".jsonl": `{"type":"assistant","isSidechain":true,"message":{"content":[{"type":"text","text":"Child agent response"}]}}
`,
	}
	for name, data := range files {
		if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	adapter := NewAdapter()
	candidates, err := adapter.Discover(t.Context())
	if err != nil || len(candidates) != 1 {
		t.Fatalf("discover = %+v, %v", candidates, err)
	}
	scanned, err := adapter.ScanFile(t.Context(), candidates[0])
	if err != nil {
		t.Fatal(err)
	}
	got := scanned.Session
	if got.Title != "Review session" || got.FirstMsg != "Inspect this screenshot [Image]" ||
		got.MsgCount != 2 || got.Model != "sonnet" || got.ToolCount != 1 ||
		strings.Contains(strings.Join(scanned.SearchParts, " "), "Internal subagent prompt") {
		t.Fatalf("unexpected main metadata: %+v", scanned)
	}
	for _, verbose := range []bool{false, true} {
		turns, err := ParseSessionMessages(path, verbose)
		if err != nil {
			t.Fatal(err)
		}
		tail, err := ParseSessionMessagesTail(path, verbose, 20, 100)
		if err != nil || len(tail) != len(turns) {
			t.Fatalf("tail = %+v, %v", tail, err)
		}
		var child, embedded, goal bool
		for _, turn := range turns {
			if turn.Role == "agent" {
				child = child || strings.Contains(turn.Text, "Child agent response")
				embedded = embedded || strings.Contains(turn.Text, "Subagent background · Assistant\nEmbedded agent response")
			}
			goal = goal || (turn.Role == "event" && strings.Contains(turn.Text, "Goal (complete) · Review done"))
		}
		if child != verbose || !embedded || !goal || turns[0].Text != "Inspect this screenshot\n[Image]" {
			t.Fatalf("unexpected preview (verbose=%v): %+v", verbose, turns)
		}
	}
}

func TestDetachedSidechainRemainsDiscoverable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "detached.jsonl")
	data := `{"type":"user","isSidechain":true,"message":{"content":"Work in background"}}
{"type":"assistant","isSidechain":true,"message":{"model":"claude-haiku","content":[{"type":"text","text":"Done"}]}}
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	scanned, err := ScanSessionFile(path, "-tmp-demo", int64(len(data)))
	if err != nil || scanned.Session.MsgCount != 2 || scanned.Session.FirstMsg != "Work in background" {
		t.Fatalf("scan = %+v, %v", scanned, err)
	}
}
