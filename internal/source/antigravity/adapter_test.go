package antigravity

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jinguo998/claude-sessions/internal/domain"
	"github.com/jinguo998/claude-sessions/internal/source"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProfileDiscoveryAndTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".gemini")
	const id = "a1b2c3d4-1111-2222-3333-444455556666"
	logs := filepath.Join("brain", id, ".system_generated", "logs")
	for _, p := range []string{"antigravity-ide", "antigravity", "antigravity-cli"} {
		put(t, filepath.Join(root, p, logs, "transcript.jsonl"), `{"type":"USER_INPUT","content":"`+p+` short log"}`)
	}
	cli := filepath.Join(root, "antigravity-cli")
	put(t, filepath.Join(cli, logs, "transcript_full.jsonl"), strings.Join([]string{
		`{"step_index":0,"type":"USER_INPUT","created_at":"2026-10-01T12:00:00Z","content":"<USER_REQUEST>\n检查配置\n</USER_REQUEST>\n<ADDITIONAL_METADATA>IDE noise</ADDITIONAL_METADATA>","media":[{"mime_type":"image/png","uri":"file:///screen.png"}]}`,
		`{"step_index":1,"type":"PLANNER_RESPONSE","created_at":"2026-10-01T12:00:01Z","model":"gemini","thinking":"internal text","content":"Answer","tool_calls":[{"name":"read_file","args":{"path":"file.go"}}],"input_tokens":5,"cache_read_tokens":7,"output_tokens":3}`,
		`{"step_index":2,"type":"GENERIC","content":"file contents"}`,
		`{"step_index":3,"type":"SYSTEM_MESSAGE","content":"internal instructions"}`,
		`{"step_index":4,"type":"USER_INPUT","created_at":"2026-10-01T12:01:00Z","content":"Follow up"}`,
		`{"type":`,
	}, "\n"))
	put(t, filepath.Join(cli, "cache", "last_conversations.json"), `{"/tmp/demo project":"`+id+`"}`)
	put(t, filepath.Join(cli, "cache", "conversation_metadata.json"), `{"conversations":{"`+id+`":{"summary":{"Title":"Picker title"}}}}`)
	put(t, filepath.Join(cli, "annotations", id+".pbtxt"), "title: \"Renamed conversation\"\n")
	ctx := t.Context()
	ide := NewIDE()
	desktop, err := ide.Discover(ctx)
	if err != nil || len(desktop) != 1 {
		t.Fatalf("IDE candidates %v, %v", desktop, err)
	}
	is, err := ide.ScanFile(ctx, desktop[0])
	if err != nil || is.Session.Source != domain.SourceAntigravityIDE || is.Session.FirstMsg != "antigravity-ide short log" {
		t.Fatalf("IDE session %#v, %v", is, err)
	}
	a := NewCLI()
	candidates, err := a.Discover(ctx)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("CLI candidates %v, %v", candidates, err)
	}
	scanned, err := a.ScanFile(ctx, candidates[0])
	s := scanned.Session
	if err != nil || s.ID != id || s.Source != domain.SourceAntigravityCLI || s.Client != "antigravity-cli" ||
		s.Title != "Renamed conversation" || s.ProjectPath != "/tmp/demo project" || s.ProjectDir != "demo project" ||
		s.FirstMsg != "检查配置 [Image]" || s.LastMsg != "Follow up" || s.Model != "gemini" || s.MsgCount != 3 || s.ToolCount != 1 ||
		s.TokenUsage != (domain.TokenUsage{Input: 12, Output: 3}) || s.Duration() != time.Minute ||
		!reflect.DeepEqual(scanned.SearchParts, []string{"检查配置\n[Image]", "Follow up"}) {
		t.Fatalf("CLI session %#v, %v", scanned, err)
	}
	for _, verbose := range []bool{false, true} {
		turns, err := a.ParsePreview(ctx, s, source.PreviewOptions{Verbose: verbose})
		wantLen := 4
		if verbose {
			wantLen++
		}
		if err != nil || len(turns) != wantLen || turns[0].Role != "user" || turns[1].Text != "Answer" ||
			turns[2].Role != "tool" || strings.Contains(turns[2].Text, "file.go") != verbose {
			t.Fatalf("verbose=%v turns %#v, %v", verbose, turns, err)
		}
		tail, err := a.ParsePreviewTail(ctx, s, source.TailOptions{Verbose: verbose, MaxMessages: 2, MaxBytes: 16})
		if err != nil || !reflect.DeepEqual(tail, turns[len(turns)-2:]) {
			t.Fatalf("tail %#v, %v", tail, err)
		}
	}
	put(t, filepath.Join(cli, "annotations", id+".pbtxt"), "title: \"Later title\"\n")
	updated, err := a.Discover(ctx)
	if err != nil || updated[0].MetadataKey == candidates[0].MetadataKey {
		t.Fatal("rename did not invalidate the session cache")
	}
}

func TestIndexWithLiveWALAndIndexedHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".gemini", "antigravity")
	// Opaque native histories are left in place; the public-facing text is in the search index.
	put(t, filepath.Join(root, "conversations", "history.pb"), "opaque")
	for _, id := range []string{"child", "archived"} {
		put(t, filepath.Join(root, "brain", id, ".system_generated", "logs", "transcript.jsonl"), `{"type":"USER_INPUT","content":"Hidden"}`)
	}
	path := filepath.Join(root, indexName)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`PRAGMA journal_mode=WAL;
CREATE TABLE conversation_summaries(conversation_id text,title text,workspace_uris text,last_modified_time datetime,parent_conversation_id text);
CREATE VIRTUAL TABLE conversations_fts USING fts5(conversation_id UNINDEXED,chunk_index UNINDEXED,archived UNINDEXED,user_body,agent_body,user_step_byte_offsets UNINDEXED,agent_step_byte_offsets UNINDEXED);
INSERT INTO conversation_summaries VALUES ('history','Indexed title','["ssh://remote/project","file:///tmp/my%20project"]','2026-10-01 12:00:00+00:00',''),('child','','[]',null,'history'),('archived','','[]',null,'');
INSERT INTO conversations_fts VALUES ('archived',-1,1,'','','[]','[]'),('history',0,0,'你好
next','first
second','[[0,0],[2,7]]','[[1,0],[3,6]]');`)
	if err != nil {
		t.Fatal(err)
	}
	a := NewIDE()
	candidates, err := a.Discover(t.Context())
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates %#v, %v", candidates, err)
	}
	scan, err := a.ScanFile(t.Context(), candidates[0])
	s := scan.Session
	if err != nil || s.ID != "history" || s.Client != "antigravity" || s.Title != "Indexed title" || s.ProjectPath != "/tmp/my project" || s.MsgCount != 4 || s.FirstMsg != "你好" || s.LastMsg != "next" || s.LastTime.Year() != 2026 {
		t.Fatalf("indexed session %#v, %v", scan, err)
	}
	turns, err := a.ParsePreview(t.Context(), s, source.PreviewOptions{})
	if err != nil || len(turns) != 4 || turns[1].Role != "assistant" || turns[1].Text != "first" || turns[2].Text != "next" {
		t.Fatalf("indexed turns %#v, %v", turns, err)
	}
	tail, err := a.ParsePreviewTail(t.Context(), s, source.TailOptions{MaxMessages: 1})
	if err != nil || len(tail) != 1 || tail[0].Text != "second" {
		t.Fatalf("tail %#v, %v", tail, err)
	}
	if _, err := db.Exec(`INSERT INTO conversations_fts VALUES ('history',1,0,'','latest','[]','[[4,0]]')`); err != nil {
		t.Fatal(err)
	}
	turns = append(turns, domain.ConversationTurn{Role: "assistant", Text: "latest"})
	for _, limit := range []int{3, 10} {
		tail, err := a.ParsePreviewTail(t.Context(), s, source.TailOptions{MaxMessages: limit})
		if err != nil || !reflect.DeepEqual(tail, turns[max(0, len(turns)-limit):]) {
			t.Fatalf("cross-chunk tail %d: %#v, %v", limit, tail, err)
		}
	}
	reader, err := openIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err = reader.Exec("DELETE FROM conversation_summaries"); err == nil {
		t.Fatal("index was writable")
	}
}

func TestMissingProfileAndUnavailableIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := NewCLI()
	if got, err := a.Discover(t.Context()); err != nil || len(got) != 0 {
		t.Fatalf("absent profile %v, %v", got, err)
	}
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	put(t, filepath.Join(root, indexName), "not a SQLite database")
	put(t, filepath.Join(root, "brain", "valid", ".system_generated", "logs", "transcript.jsonl"), `{"type":"USER_INPUT","content":"still readable"}`)
	got, err := a.Discover(t.Context())
	if err == nil || len(got) != 1 {
		t.Fatalf("partial discovery %v, %v", got, err)
	}
}

func TestClientHandoff(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	project := filepath.Join(t.TempDir(), "a project")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	for _, launcher := range []string{"antigravity-ide", "antigravity"} {
		f := filepath.Join(os.Getenv("PATH"), launcher)
		if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		a           Adapter
		client, exe string
		args        []string
	}{
		{NewCLI(), "antigravity-cli", "agy", []string{"agy", "--conversation", "session-1"}},
		{NewIDE(), "antigravity-ide", "antigravity-ide", []string{"antigravity-ide", project}},
		{NewIDE(), "antigravity", "antigravity", []string{"antigravity", project}},
	} {
		target := domain.ResumeTarget{Session: domain.Session{ID: "session-1", Client: tc.client, Source: tc.a.Source(), ProjectPath: project, Title: "A title"}, Action: domain.ResumeActionResume}
		plan, err := tc.a.PlanResume(t.Context(), target)
		if err != nil || plan.Executable != tc.exe || !reflect.DeepEqual(plan.Args, tc.args) || plan.WorkingDir != target.Session.ProjectPath || !strings.Contains(plan.Message, target.Session.ID) {
			t.Fatalf("%s plan %#v, %v", tc.client, plan, err)
		}
		target.Action = domain.ResumeActionCd
		if p, err := tc.a.PlanResume(t.Context(), target); err != nil || !p.CdOnly || p.HandoffDir != target.Session.ProjectPath {
			t.Fatalf("cd %#v, %v", p, err)
		}
		target.Action = domain.ResumeActionFork
		if _, err := tc.a.PlanResume(t.Context(), target); !errors.Is(err, source.ErrUnsupportedCapability) {
			t.Fatalf("fork returned %v", err)
		}
	}
	cli := NewCLI()
	missing := domain.Session{ID: "detached", ProjectPath: filepath.Join(project, "deleted")}
	plan, err := cli.PlanResume(t.Context(), domain.ResumeTarget{Session: missing, Action: domain.ResumeActionResume})
	if err != nil || plan.WorkingDir != "" || !reflect.DeepEqual(plan.Args, []string{"agy", "--conversation", "detached"}) {
		t.Fatalf("removed workspace handoff %#v, %v", plan, err)
	}
}
