package model

import (
	"fmt"
	"sort"
	"time"

	"github.com/jinguo998/agent-sessions/internal/domain"
)

type Source = domain.Source
type TokenUsage = domain.TokenUsage

const (
	SourceClaude         = domain.SourceClaude
	SourceCodex          = domain.SourceCodex
	SourceOpenCode       = domain.SourceOpenCode
	SourceAntigravityIDE = domain.SourceAntigravityIDE
	SourceAntigravityCLI = domain.SourceAntigravityCLI
)

// Session is the application-facing session view model. It embeds the pure
// domain session and carries derived search corpus outside the domain model.
type Session struct {
	domain.Session
	SearchText string
}

func FromDomain(sess domain.Session, searchText string) Session {
	return Session{
		Session:    sess,
		SearchText: searchText,
	}
}

func (s Session) Domain() domain.Session {
	return s.Session
}

func ToDomainSessions(sessions []Session) []domain.Session {
	out := make([]domain.Session, len(sessions))
	for i, sess := range sessions {
		out[i] = sess.Domain()
	}
	return out
}

func (s Session) FormatDuration() string {
	d := s.Duration()
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func (s Session) FormatSize() string {
	switch {
	case s.FileSize < 1024:
		return fmt.Sprintf("%dB", s.FileSize)
	case s.FileSize < 1024*1024:
		return fmt.Sprintf("%.0fKB", float64(s.FileSize)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(s.FileSize)/(1024*1024))
	}
}

func SortRecent(sessions []Session) {
	sort.Slice(sessions, func(i, j int) bool {
		return domain.LessLastTime(sessions[i].Session, sessions[j].Session)
	})
}

func SortByMessages(sessions []Session) {
	sort.Slice(sessions, func(i, j int) bool {
		return domain.LessMsgCount(sessions[i].Session, sessions[j].Session)
	})
}

func SortByTools(sessions []Session) {
	sort.Slice(sessions, func(i, j int) bool {
		return domain.LessToolCount(sessions[i].Session, sessions[j].Session)
	})
}

func SortByTokens(sessions []Session) {
	sort.Slice(sessions, func(i, j int) bool {
		return domain.LessTotalTokens(sessions[i].Session, sessions[j].Session)
	})
}
