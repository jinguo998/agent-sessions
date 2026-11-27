package antigravity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/jinguo998/agent-sessions/internal/domain"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

const indexName = "conversation_summaries.db"

// A background reindex can change message text without touching the native history.
func indexFingerprint(path string) string {
	var stamp strings.Builder
	for _, file := range []string{path, path + "-wal"} {
		if info, err := os.Stat(file); err == nil {
			fmt.Fprintf(&stamp, "/%d:%d", info.Size(), info.ModTime().UnixNano())
		}
	}
	return stamp.String()
}

type summary struct {
	Title, Workspace, Updated string
	Hidden, Indexed           bool
}

func localWorkspace(uris []string) string {
	for _, raw := range uris {
		if filepath.IsAbs(raw) {
			return filepath.Clean(raw)
		}
		u, err := url.Parse(raw)
		if err == nil && u.Scheme == "file" && (u.Host == "" || strings.EqualFold(u.Host, "localhost")) && filepath.IsAbs(u.Path) {
			return filepath.Clean(u.Path)
		}
	}
	return ""
}

func openIndex(path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	params := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(1000)"}}
	// An idle WAL database may be in a read-only directory without its
	// sidecars. Keep ordinary SQLite locking for rollback-mode databases.
	if _, err := os.Stat(path + "-wal"); errors.Is(err, os.ErrNotExist) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		var versions [2]byte
		_, err = f.ReadAt(versions[:], 18) // SQLite header write/read versions.
		f.Close()
		if err == nil && versions == [2]byte{2, 2} {
			params.Set("immutable", "1")
		}
	}
	u.RawQuery = params.Encode()
	return sql.Open("sqlite3", u.String())
}

func readIndex(ctx context.Context, root string, entries map[string]summary) error {
	path := filepath.Join(root, indexName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	db, err := openIndex(path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Older indexes omit parent_conversation_id.
	var hasParent bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pragma_table_info('conversation_summaries') WHERE name='parent_conversation_id')").Scan(&hasParent); err != nil {
		return err
	}
	parent := "''"
	if hasParent {
		parent = "coalesce(parent_conversation_id,'')"
	}
	rows, err := tx.QueryContext(ctx, "SELECT conversation_id, coalesce(title,''), coalesce(workspace_uris,''), coalesce(cast(last_modified_time AS text),''), "+parent+" FROM conversation_summaries")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, rawURIs, updated, parentID string
		if err := rows.Scan(&id, &title, &rawURIs, &updated, &parentID); err != nil {
			return err
		}
		s := entries[id]
		if title != "" {
			s.Title = title
		}
		var uris []string
		if json.Unmarshal([]byte(rawURIs), &uris) == nil {
			if path := localWorkspace(uris); path != "" {
				s.Workspace = path
			}
		}
		s.Hidden = s.Hidden || parentID != ""
		s.Updated = updated
		entries[id] = s
	}
	if err := rows.Err(); err != nil {
		return err
	}
	var hasArchive bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pragma_table_info('conversations_fts') WHERE name='archived')").Scan(&hasArchive); err != nil || !hasArchive {
		return err
	}
	headers, err := tx.QueryContext(ctx, "SELECT conversation_id, max(chunk_index=-1 AND coalesce(archived,0)=1), max(chunk_index>=0) FROM conversations_fts GROUP BY conversation_id")
	if err != nil {
		return err
	}
	defer headers.Close()
	for headers.Next() {
		var id string
		var archived, indexed bool
		if err := headers.Scan(&id, &archived, &indexed); err != nil {
			return err
		}
		s := entries[id]
		s.Hidden = s.Hidden || archived
		s.Indexed = indexed
		entries[id] = s
	}
	return headers.Err()
}

// The CLI picker also keeps titles and last-used working directories in JSON.
func readPickerCache(root string, entries map[string]summary) {
	var metadata struct {
		Conversations map[string]struct {
			Internal bool `json:"is_internal"`
			Summary  struct {
				Title         string
				WorkspaceURIs []string
				Internal      bool
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "cache", "conversation_metadata.json")); err == nil && json.Unmarshal(data, &metadata) == nil {
		for id, row := range metadata.Conversations {
			entries[id] = summary{Title: row.Summary.Title, Workspace: localWorkspace(row.Summary.WorkspaceURIs), Hidden: row.Internal || row.Summary.Internal}
		}
	}
	var last map[string]string
	if data, err := os.ReadFile(filepath.Join(root, "cache", "last_conversations.json")); err == nil && json.Unmarshal(data, &last) == nil {
		paths := make([]string, 0, len(last))
		for path := range last {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			id := last[path]
			s := entries[id]
			if s.Workspace == "" {
				s.Workspace = localWorkspace([]string{path})
				entries[id] = s
			}
		}
	}
}

// Antigravity's search index stores the user and agent bodies separately.
// The pairs contain the original step number and a UTF-8 byte offset.
func indexedTurns(ctx context.Context, path, id string, tail int) ([]domain.ConversationTurn, error) {
	db, err := openIndex(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := `SELECT coalesce(user_body,''), coalesce(user_step_byte_offsets,'[]'),
		coalesce(agent_body,''), coalesce(agent_step_byte_offsets,'[]')
		FROM conversations_fts WHERE conversation_id=? AND chunk_index>=0 ORDER BY cast(chunk_index AS integer)`
	if tail > 0 {
		query += " DESC"
	}
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []domain.ConversationTurn
	for rows.Next() {
		var userBody, agentBody string
		var userOffsets, agentOffsets []byte
		if err := rows.Scan(&userBody, &userOffsets, &agentBody, &agentOffsets); err != nil {
			return nil, err
		}
		type step struct {
			index int
			turn  domain.ConversationTurn
		}
		var chunk []step
		appendBody := func(role, body string, offsets []byte) error {
			var pairs [][2]int
			if err := json.Unmarshal(offsets, &pairs); err != nil {
				return err
			}
			for i, pair := range pairs {
				end := len(body)
				if i+1 < len(pairs) {
					end = pairs[i+1][1]
				}
				if pair[1] < 0 || pair[1] > end || end > len(body) {
					return errors.New("invalid Antigravity search index byte offset")
				}
				if text := strings.TrimSpace(body[pair[1]:end]); text != "" {
					chunk = append(chunk, step{pair[0], domain.ConversationTurn{Role: role, Text: text}})
				}
			}
			return nil
		}
		if err := appendBody("user", userBody, userOffsets); err != nil {
			return nil, err
		}
		if err := appendBody("assistant", agentBody, agentOffsets); err != nil {
			return nil, err
		}
		sort.SliceStable(chunk, func(i, j int) bool { return chunk[i].index < chunk[j].index })
		if tail > 0 {
			slices.Reverse(chunk)
		}
		for _, step := range chunk {
			turns = append(turns, step.turn)
			if tail > 0 && len(turns) == tail {
				slices.Reverse(turns)
				return turns, nil
			}
		}
	}
	if tail > 0 {
		slices.Reverse(turns)
	}
	return turns, rows.Err()
}
