package antigravity

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jinguo998/claude-sessions/internal/domain"
	"github.com/jinguo998/claude-sessions/internal/source"
)

type record struct {
	Type      string `json:"type"`
	Created   string `json:"created_at"`
	Content   string `json:"content"`
	Model     string `json:"model"`
	Error     string `json:"error"`
	Input     int    `json:"input_tokens"`
	CacheRead int    `json:"cache_read_tokens"`
	Output    int    `json:"output_tokens"`
	Media     []struct {
		MIME string `json:"mime_type"`
	} `json:"media"`
	Tools []struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"tool_calls"`
}

func (s record) timestamp() time.Time {
	ts, _ := time.Parse(time.RFC3339Nano, s.Created)
	return ts
}

func (s record) userText() string {
	text := strings.TrimSpace(s.Content)
	if request, ok := strings.CutPrefix(text, "<USER_REQUEST>"); ok {
		text, _, _ = strings.Cut(request, "</USER_REQUEST>")
		text = strings.TrimSpace(text)
	}
	for _, m := range s.Media {
		label := "[Attachment]"
		if strings.HasPrefix(m.MIME, "image/") {
			label = "[Image]"
		}
		text = strings.TrimSpace(text + "\n" + label)
	}
	return text
}

func (s record) appendTurns(turns []domain.ConversationTurn, verbose bool) []domain.ConversationTurn {
	ts := s.timestamp()
	add := func(role, text string) {
		if text = strings.TrimSpace(text); text != "" {
			turns = append(turns, domain.ConversationTurn{Role: role, Text: text, Timestamp: ts})
		}
	}
	switch s.Type {
	case "USER_INPUT":
		add("user", s.userText())
	case "PLANNER_RESPONSE":
		add("assistant", s.Content)
		for _, tool := range s.Tools {
			label := tool.Name
			if verbose && len(tool.Args) > 0 && string(tool.Args) != "null" {
				label += " " + string(tool.Args)
			}
			add("tool", label)
		}
	case "GENERIC":
		if verbose {
			add("tool_result", s.Content)
			add("event", s.Error)
		}
	case "ERROR_MESSAGE":
		if s.Error != "" {
			add("event", s.Error)
		} else {
			add("event", s.Content)
		}
	}
	return turns
}

func readTranscript(ctx context.Context, path string, visit func(record)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := r.ReadBytes('\n')
		var step record
		if len(line) > 0 && json.Unmarshal(line, &step) == nil {
			visit(step)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (a Adapter) ParsePreview(ctx context.Context, s domain.Session, opts source.PreviewOptions) ([]domain.ConversationTurn, error) {
	if index := s.Attributes["index"]; index != "" {
		return indexedTurns(ctx, index, s.ID, 0)
	}
	var turns []domain.ConversationTurn
	err := readTranscript(ctx, s.FilePath, func(step record) { turns = step.appendTurns(turns, opts.Verbose) })
	return turns, err
}

func (a Adapter) ParsePreviewTail(ctx context.Context, s domain.Session, opts source.TailOptions) ([]domain.ConversationTurn, error) {
	if err := ctx.Err(); err != nil || opts.MaxMessages <= 0 {
		return nil, err
	}
	if index := s.Attributes["index"]; index != "" {
		return indexedTurns(ctx, index, s.ID, opts.MaxMessages)
	}
	return source.ParseTailMessages(s.FilePath, opts.Verbose, opts.MaxMessages, opts.MaxBytes, func(turns []domain.ConversationTurn, line []byte, verbose bool) []domain.ConversationTurn {
		var step record
		if json.Unmarshal(line, &step) != nil {
			return turns
		}
		return step.appendTurns(turns, verbose)
	})
}
