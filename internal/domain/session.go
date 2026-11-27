package domain

import (
	"strings"
	"time"
)

// Source identifies the client that owns a session.
type Source string

const (
	SourceClaude         Source = "claude"
	SourceCodex          Source = "codex"
	SourceOpenCode       Source = "opencode"
	SourceAntigravityIDE Source = "antigravity-ide"
	SourceAntigravityCLI Source = "antigravity-cli"
)

// TokenUsage holds source-normalized token accounting.
type TokenUsage struct {
	Input  int
	Output int
}

func (u TokenUsage) Total() int {
	return u.Input + u.Output
}

// ConversationTurn is one displayable message, tool action, or event in a session preview.
type ConversationTurn struct {
	Role      string
	Text      string
	Timestamp time.Time
}

type Project struct {
	Dir  string
	Path string
}

type ResumeAction string

const (
	ResumeActionResume ResumeAction = "resume"
	ResumeActionFork   ResumeAction = "fork"
	ResumeActionCd     ResumeAction = "cd"
)

type PermissionMode string

const (
	PermissionModeSafe PermissionMode = "safe"
	PermissionModeFast PermissionMode = "fast"
)

type ResumeTarget struct {
	Session        Session
	Action         ResumeAction
	PermissionMode PermissionMode
}

type ResumePlan struct {
	WorkingDir string
	Executable string
	Args       []string
	HandoffDir string
	Message    string
	CdOnly     bool
}

// Session holds source-agnostic metadata for one conversation.
type Session struct {
	ID          string
	Source      Source
	ProjectDir  string
	ProjectPath string
	Title       string
	Client      string
	Origin      string
	Labels      []string
	Attributes  map[string]string
	FirstMsg    string
	LastMsg     string
	StartTime   time.Time
	LastTime    time.Time
	MsgCount    int
	ToolCount   int
	FileSize    int64
	FilePath    string
	Model       string
	TokenUsage  TokenUsage
}

// Duration returns the session duration.
func (s Session) Duration() time.Duration {
	if s.StartTime.IsZero() || s.LastTime.IsZero() {
		return 0
	}
	return s.LastTime.Sub(s.StartTime)
}

// TotalTokens returns combined input and output token usage.
func (s Session) TotalTokens() int {
	return s.TokenUsage.Total()
}

// ProjectShortName returns the last segment of the decoded project path.
func (s Session) ProjectShortName() string {
	path := s.ProjectPath
	if path == "" {
		return s.ProjectDir
	}
	path = strings.TrimRight(path, "/")
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

// LessLastTime reports whether a is more recent than b.
func LessLastTime(a, b Session) bool { return a.LastTime.After(b.LastTime) }

// LessMsgCount reports whether a has more messages than b.
func LessMsgCount(a, b Session) bool { return a.MsgCount > b.MsgCount }

// LessToolCount reports whether a has more tool calls than b.
func LessToolCount(a, b Session) bool { return a.ToolCount > b.ToolCount }

// LessTotalTokens reports whether a used more total tokens than b.
func LessTotalTokens(a, b Session) bool { return a.TotalTokens() > b.TotalTokens() }

// SortByLastTime sorts sessions by LastTime descending.
type SortByLastTime []Session

func (s SortByLastTime) Len() int           { return len(s) }
func (s SortByLastTime) Less(i, j int) bool { return LessLastTime(s[i], s[j]) }
func (s SortByLastTime) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// SortByMsgCount sorts sessions by MsgCount descending.
type SortByMsgCount []Session

func (s SortByMsgCount) Len() int           { return len(s) }
func (s SortByMsgCount) Less(i, j int) bool { return LessMsgCount(s[i], s[j]) }
func (s SortByMsgCount) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// SortByToolCount sorts sessions by ToolCount descending.
type SortByToolCount []Session

func (s SortByToolCount) Len() int           { return len(s) }
func (s SortByToolCount) Less(i, j int) bool { return LessToolCount(s[i], s[j]) }
func (s SortByToolCount) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// SortByTotalTokens sorts sessions by total token usage descending.
type SortByTotalTokens []Session

func (s SortByTotalTokens) Len() int           { return len(s) }
func (s SortByTotalTokens) Less(i, j int) bool { return LessTotalTokens(s[i], s[j]) }
func (s SortByTotalTokens) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
