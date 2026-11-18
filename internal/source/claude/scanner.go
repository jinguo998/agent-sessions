package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jinguo998/claude-sessions/internal/domain"
	"github.com/jinguo998/claude-sessions/internal/source"
)

type Adapter struct{}

func NewAdapter() Adapter {
	return Adapter{}
}

func (Adapter) Source() domain.Source {
	return domain.SourceClaude
}

func (a Adapter) Discover(ctx context.Context) ([]source.Candidate, error) {
	return discoverSessions(ctx)
}

func (a Adapter) ScanFile(ctx context.Context, candidate source.Candidate) (source.ScannedSession, error) {
	_ = ctx
	return ScanSessionFile(candidate.Path, candidate.ProjectDir, candidate.FileSize)
}

// ScanSessionFile reads a single JSONL file and extracts session metadata.
func ScanSessionFile(path, projectDir string, fileSize int64) (source.ScannedSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return source.ScannedSession{}, err
	}
	defer f.Close()

	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	var startTime, lastTime time.Time
	var customTitle, aiTitle string
	var main, sidechain conversationStats

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB max line, matching parser

	for sc.Scan() {
		var line jsonLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}

		// Collect timestamps from top-level and snapshot fields.
		timestamps := []string{}
		if line.Timestamp != "" {
			timestamps = append(timestamps, line.Timestamp)
		}
		if line.Snapshot != nil && line.Snapshot.Timestamp != "" {
			timestamps = append(timestamps, line.Snapshot.Timestamp)
		}
		for _, raw := range timestamps {
			if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				if startTime.IsZero() || ts.Before(startTime) {
					startTime = ts
				}
				if ts.After(lastTime) {
					lastTime = ts
				}
			}
		}

		switch line.Type {
		case "custom-title":
			customTitle = strings.TrimSpace(line.CustomTitle)
		case "ai-title":
			aiTitle = strings.TrimSpace(line.AITitle)
		case "user", "assistant":
			if line.IsSidechain {
				sidechain.add(line)
			} else {
				main.add(line)
			}
		}
	}

	title := customTitle
	if title == "" {
		title = aiTitle
	}
	stats := main
	// Detached agent transcripts can contain only sidechain records.
	if stats.msgCount == 0 || (len(stats.userMessages) == 0 && stats.model == "" && len(sidechain.userMessages) > 0) {
		stats = sidechain
	}
	if title != "" {
		stats.userMessages = append(stats.userMessages, title)
	}

	projectPath := decodeProjectDirCached(projectDir)

	return source.ScannedSession{
		Session: domain.Session{
			ID:          id,
			ProjectDir:  projectDir,
			ProjectPath: projectPath,
			Title:       title,
			FirstMsg:    stats.firstMsg,
			LastMsg:     stats.lastMsg,
			StartTime:   startTime,
			LastTime:    lastTime,
			MsgCount:    stats.msgCount,
			ToolCount:   stats.toolCount,
			FileSize:    fileSize,
			FilePath:    path,
			Model:       stats.model,
			TokenUsage:  stats.usage,
			Source:      domain.SourceClaude,
		},
		SearchParts: stats.userMessages,
	}, nil
}

type conversationStats struct {
	firstMsg     string
	lastMsg      string
	lastRole     string
	model        string
	msgCount     int
	toolCount    int
	usage        domain.TokenUsage
	userMessages []string
	seenUsageIDs map[string]struct{}
}

func (s *conversationStats) add(line jsonLine) {
	if line.Type == "user" && line.IsMeta {
		return
	}
	// Consecutive records with the same role form one conversation turn.
	if line.Type != s.lastRole {
		s.msgCount++
		s.lastRole = line.Type
	}
	if line.Message == nil {
		return
	}
	if line.Type == "user" {
		text := strings.Join(strings.Fields(claudeUserText(line.Message.Content)), " ")
		if text == "" {
			return
		}
		s.userMessages = append(s.userMessages, text)
		if runes := []rune(text); len(runes) > 100 {
			text = string(runes[:100])
		}
		if s.firstMsg == "" {
			s.firstMsg = text
		}
		s.lastMsg = text
		return
	}

	if s.model == "" {
		s.model = line.Message.Model
		for _, name := range []string{"opus", "sonnet", "haiku"} {
			if strings.Contains(s.model, name) {
				s.model = name
				break
			}
		}
	}
	var blocks []contentItem
	if json.Unmarshal(line.Message.Content, &blocks) == nil {
		for _, block := range blocks {
			if block.Type == "tool_use" {
				s.toolCount++
			}
		}
	}
	// Thinking, text and tool records can repeat the usage for one message ID.
	if usage := line.Message.Usage; usage != nil {
		id := line.Message.ID
		if _, seen := s.seenUsageIDs[id]; seen {
			return
		}
		if id != "" {
			if s.seenUsageIDs == nil {
				s.seenUsageIDs = make(map[string]struct{})
			}
			s.seenUsageIDs[id] = struct{}{}
		}
		s.usage.Input += usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		s.usage.Output += usage.OutputTokens
	}
}

func discoverSessions(ctx context.Context) ([]source.Candidate, error) {
	root := os.Getenv("CLAUDE_CONFIG_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(home, ".claude")
	}
	baseDir := filepath.Join(root, "projects")

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var candidates []source.Candidate
	var firstErr error
	for _, projEntry := range entries {
		if err := ctx.Err(); err != nil {
			return candidates, err
		}
		if !projEntry.IsDir() {
			continue
		}
		projDir := projEntry.Name()
		projPath := filepath.Join(baseDir, projDir)

		files, err := os.ReadDir(projPath)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			filePath := filepath.Join(projPath, f.Name())
			info, err := f.Info()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			candidates = append(candidates, source.Candidate{
				Source:      domain.SourceClaude,
				Path:        filePath,
				ProjectDir:  projDir,
				MetadataKey: "claude-meta-v2",
				FileSize:    info.Size(),
				ModTime:     info.ModTime(),
			})
		}
	}
	return candidates, firstErr
}

const maxWorkers = 16

func ScanAllSessions(ctx context.Context) ([]source.ScannedSession, error) {
	candidates, err := discoverSessions(ctx)
	if err != nil {
		return nil, err
	}
	type result struct {
		session source.ScannedSession
		err     error
	}
	var wg sync.WaitGroup
	ch := make(chan result, 100)
	sem := make(chan struct{}, maxWorkers)
	for _, candidate := range candidates {
		wg.Add(1)
		go func(candidate source.Candidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sess, err := ScanSessionFile(candidate.Path, candidate.ProjectDir, candidate.FileSize)
			ch <- result{sess, err}
		}(candidate)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var sessions []source.ScannedSession
	for r := range ch {
		if r.err == nil && r.session.Session.MsgCount > 0 {
			sessions = append(sessions, r.session)
		}
	}
	return sessions, nil
}
