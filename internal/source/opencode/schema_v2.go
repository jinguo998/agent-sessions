package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jinguo998/agent-sessions/internal/domain"
)

const schemaV2 = "v2"

func (a Adapter) schema(ctx context.Context, dbPath string) (string, error) {
	var tables []struct {
		Name string `json:"name"`
	}
	if err := a.dbQuery(ctx, dbPath, "select name from sqlite_schema where type = 'table' and name = 'session_v2'", &tables); err != nil {
		return "", err
	}
	if len(tables) > 0 {
		return schemaV2, nil
	}
	return "v1", nil
}

const discoverSQLV2 = `
select
  s.id, s.directory, s.title, s.agent, s.model, s.tokens_input, s.tokens_output,
  s.time_created, s.time_updated,
  coalesce((select max(time_updated) from session_message where session_id = s.id), 0) as message_updated,
  (select count(*) from session_message where session_id = s.id) as message_count,
  (select count(*) from session_message where session_id = s.id and type = 'user') as user_count,
  (select count(*) from session_message m, json_each(m.data, '$.content') p
   where m.session_id = s.id and m.type = 'assistant' and p.value ->> 'type' = 'tool') as tool_count
from session_v2 s
where s.time_archived is null and s.parent_id is null
order by s.time_updated desc, s.id desc`

func userTextsSQLV2(id string) string {
	return `
select time_created, data ->> 'text' as text
from session_message
where session_id = ` + sqlQuote(id) + ` and type = 'user'
order by seq, id`
}

func previewSQLV2(id string, limit int) string {
	query := `
select id as message_id, type as role, time_created as message_time, data as message_data, seq
from session_message
where session_id = ` + sqlQuote(id) + ` and type in ('user', 'assistant')`
	if limit <= 0 {
		return query + " order by seq, id"
	}
	return "select * from (" + query + fmt.Sprintf(" order by seq desc, id desc limit %d", limit) +
		") order by seq, message_id"
}

func appendV2Message(turns []domain.ConversationTurn, row previewRow, data string, verbose bool) []domain.ConversationTurn {
	var message struct {
		Text    string `json:"text"`
		Content []struct {
			partData
			Name string `json:"name"`
		} `json:"content"`
		Files []struct {
			Filename string `json:"filename"`
			Mime     string `json:"mime"`
		} `json:"files"`
	}
	if json.Unmarshal([]byte(data), &message) != nil {
		return turns
	}
	if row.Role == "user" {
		parts := []string{strings.TrimSpace(message.Text)}
		for _, file := range message.Files {
			label := "File"
			if strings.HasPrefix(file.Mime, "image/") {
				label = "Image"
			}
			if file.Filename != "" {
				label += ": " + file.Filename
			}
			parts = append(parts, "["+label+"]")
		}
		return appendPreviewPart(turns, row, partData{Type: "text", Text: strings.Join(parts, "\n")}, verbose)
	}
	for _, block := range message.Content {
		part := block.partData
		if part.Tool == "" {
			part.Tool = block.Name
		}
		turns = appendPreviewPart(turns, row, part, verbose)
	}
	return turns
}
