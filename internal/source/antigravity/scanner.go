package antigravity

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jinguo998/agent-sessions/internal/domain"
	"github.com/jinguo998/agent-sessions/internal/source"
)

type Adapter struct{ kind domain.Source }

func NewIDE() Adapter                   { return Adapter{domain.SourceAntigravityIDE} }
func NewCLI() Adapter                   { return Adapter{domain.SourceAntigravityCLI} }
func (a Adapter) Source() domain.Source { return a.kind }

func (a Adapter) Discover(ctx context.Context) ([]source.Candidate, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	profiles := []string{"antigravity-cli"}
	if a.kind == domain.SourceAntigravityIDE {
		// The original IDE and the desktop App use the second directory.
		profiles = []string{"antigravity-ide", "antigravity"}
	}
	var candidates []source.Candidate
	var warnings []error
	seen := make(map[string]bool)
	for _, profile := range profiles {
		root := filepath.Join(home, ".gemini", profile)
		entries := make(map[string]summary)
		if profile == "antigravity-cli" {
			readPickerCache(root, entries)
		}
		if err := readIndex(ctx, root, entries); err != nil {
			warnings = append(warnings, fmt.Errorf("%s index: %w", profile, err))
		}
		dirs, err := os.ReadDir(filepath.Join(root, "brain"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			warnings = append(warnings, err)
		}
		for _, d := range dirs {
			if _, known := entries[d.Name()]; d.IsDir() && !known {
				entries[d.Name()] = summary{}
			}
		}
		index := filepath.Join(root, indexName)
		indexStamp := ""
		for id, meta := range entries {
			if err := ctx.Err(); err != nil {
				return candidates, err
			}
			if seen[id] || meta.Hidden || !filepath.IsLocal(id) || filepath.Base(id) != id || id == "." {
				continue
			}
			path, info, err := historyFile(root, id, meta.Indexed)
			if err != nil {
				warnings = append(warnings, err)
			}
			if info == nil {
				continue
			}
			if title := annotationTitle(root, id); title != "" {
				meta.Title = title
			}
			attrs := map[string]string{"id": id, "profile": profile, "title": meta.Title, "updated": meta.Updated}
			stamp := ""
			if filepath.Ext(path) != ".jsonl" {
				attrs["index"] = index
				if indexStamp == "" {
					indexStamp = indexFingerprint(index)
				}
				stamp = indexStamp
			}
			key, _ := json.Marshal([]string{meta.Title, meta.Workspace, meta.Updated, stamp})
			candidates = append(candidates, source.Candidate{
				Source: a.kind, Path: path, ProjectDir: meta.Workspace, FileSize: info.Size(),
				ModTime: info.ModTime(), MetadataKey: string(key), Attributes: attrs,
			})
			seen[id] = true
		}
	}
	return candidates, errors.Join(warnings...)
}

func historyFile(root, id string, indexed bool) (string, os.FileInfo, error) {
	logs := filepath.Join(root, "brain", id, ".system_generated", "logs")
	paths := []string{filepath.Join(logs, "transcript_full.jsonl"), filepath.Join(logs, "transcript.jsonl")}
	if indexed {
		paths = append(paths, filepath.Join(root, "conversations", id+".db"), filepath.Join(root, "conversations", id+".pb"))
	}
	var warning error
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return path, info, warning
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			warning = errors.Join(warning, err)
		}
	}
	return "", nil, warning
}

func annotationTitle(root, id string) string {
	f, err := os.Open(filepath.Join(root, "annotations", id+".pbtxt"))
	if err != nil {
		return ""
	}
	defer f.Close()
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		if value, ok := strings.CutPrefix(lines.Text(), "title:"); ok {
			title, _ := strconv.Unquote(strings.TrimSpace(value))
			return title
		}
	}
	return ""
}

func (a Adapter) ScanFile(ctx context.Context, c source.Candidate) (source.ScannedSession, error) {
	s := domain.Session{ID: c.Attributes["id"], Source: a.kind, Client: c.Attributes["profile"],
		ProjectPath: c.ProjectDir, Title: c.Attributes["title"], FileSize: c.FileSize, FilePath: c.Path}
	if s.ProjectPath != "" {
		s.ProjectDir = filepath.Base(s.ProjectPath)
	}
	var prompts []string
	addMessage := func(role, text string) {
		if text == "" {
			return
		}
		s.MsgCount++
		if role == "user" {
			prompts = append(prompts, text)
			label := strings.Join(strings.Fields(text), " ")
			if runes := []rune(label); len(runes) > 100 {
				label = string(runes[:100])
			}
			if s.FirstMsg == "" {
				s.FirstMsg = label
			}
			s.LastMsg = label
		}
	}
	if index := c.Attributes["index"]; index != "" {
		s.Attributes = map[string]string{"index": index}
		turns, err := indexedTurns(ctx, index, s.ID, 0)
		if err != nil {
			return source.ScannedSession{}, err
		}
		for _, turn := range turns {
			addMessage(turn.Role, turn.Text)
		}
	} else {
		err := readTranscript(ctx, c.Path, func(step record) {
			ts := step.timestamp()
			if !ts.IsZero() && (s.StartTime.IsZero() || ts.Before(s.StartTime)) {
				s.StartTime = ts
			}
			if ts.After(s.LastTime) {
				s.LastTime = ts
			}
			switch step.Type {
			case "USER_INPUT":
				addMessage("user", step.userText())
			case "PLANNER_RESPONSE":
				addMessage("assistant", strings.TrimSpace(step.Content))
				s.ToolCount += len(step.Tools)
				if step.Model != "" {
					s.Model = step.Model
				}
			}
			if step.Type == "PLANNER_RESPONSE" || step.Type == "CHECKPOINT" {
				s.TokenUsage.Input += step.Input + step.CacheRead
				s.TokenUsage.Output += step.Output
			}
		})
		if err != nil {
			return source.ScannedSession{}, err
		}
	}
	if s.LastTime.IsZero() {
		s.LastTime, _ = time.Parse(time.RFC3339Nano, strings.Replace(c.Attributes["updated"], " ", "T", 1))
		if s.LastTime.IsZero() {
			s.LastTime = c.ModTime
		}
	}
	return source.ScannedSession{Session: s, SearchParts: prompts}, nil
}
