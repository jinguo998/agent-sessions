package opencode

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinguo998/claude-sessions/internal/source"
)

func TestV2ReadOnlySessionDiscoveryAndPreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "open code #test.db")
	writer, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	_, err = writer.Exec(`
pragma journal_mode = WAL;
create table session_v2(id text, directory text, title text, agent text,
  model text, tokens_input integer, tokens_output integer, time_created integer,
  time_updated integer, time_archived integer, parent_id text);
create table session_message(id text, session_id text, type text, seq integer,
  time_created integer, time_updated integer, data text);
insert into session_v2 values
 ('main','/tmp/demo','Review','build','{"id":"demo","providerID":"test"}',15,4,1,6,null,null),
 ('child','/tmp/demo','Child',null,null,0,0,1,6,null,'main'),
 ('archived','/tmp/demo','Archived',null,null,0,0,1,6,6,null);
insert into session_message values
 ('msg2','main','user',1,2,2,'{"text":"Review this change","files":[{"filename":"preview.png","mime":"image/png"}]}'),
 ('msg3','main','assistant',2,2,3,'{"content":[{"type":"tool","name":"bash","state":{"input":{"command":"git diff"},"content":[{"type":"text","text":"diff output"}]}}]}'),
 ('msg1','main','assistant',3,2,4,'{"content":[{"type":"text","text":"Looks good"}]}'),
 ('msg4','main','idle',4,6,6,'{}');
`)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", path)
	a := NewAdapter()
	candidates, err := a.Discover(t.Context())
	if err != nil || len(candidates) != 1 {
		t.Fatalf("discover = %+v, %v", candidates, err)
	}
	scanned, err := a.ScanFile(t.Context(), candidates[0])
	if err != nil {
		t.Fatal(err)
	}
	sess := scanned.Session
	if sess.ID != "main" || sess.MsgCount != 1 || sess.ToolCount != 1 || sess.Model != "test/demo" ||
		sess.FirstMsg != "Review this change" || sess.TotalTokens() != 19 {
		t.Fatalf("scan = %+v", scanned)
	}
	turns, err := a.ParsePreview(t.Context(), sess, source.PreviewOptions{Verbose: true})
	if err != nil || len(turns) != 4 || !strings.Contains(turns[0].Text, "[Image: preview.png]") ||
		turns[1].Text != "bash git diff" || turns[2].Text != "diff output" || turns[3].Text != "Looks good" {
		t.Fatalf("full preview = %+v, %v", turns, err)
	}
	turns, err = a.ParsePreviewTail(t.Context(), sess, source.TailOptions{MaxMessages: 2})
	if err != nil || len(turns) != 2 || turns[0].Text != "bash git diff" || turns[1].Text != "Looks good" {
		t.Fatalf("tail preview = %+v, %v", turns, err)
	}
	var ignored any
	if err := (sqliteRunner{}).Query(t.Context(), path, "delete from session_v2", &ignored); err == nil {
		t.Fatal("database connection allowed writing to the OpenCode database")
	}
}
