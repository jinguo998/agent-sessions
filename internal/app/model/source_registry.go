package model

import (
	"strings"

	"github.com/jinguo998/agent-sessions/internal/domain"
)

type PermissionMode = domain.PermissionMode

const (
	PermissionModeSafe = domain.PermissionModeSafe
	PermissionModeFast = domain.PermissionModeFast
)

// SourceInfo describes terminal-agnostic source metadata and user-facing
// action capabilities. Visual styling (such as terminal colors) belongs in UI.
type SourceInfo struct {
	Source                   Source
	Label                    string
	Badge                    string
	ResumeLabel              string
	DefaultPermissionMode    PermissionMode
	SupportsSafeResumeAction bool
	SupportsFork             bool
	SupportsArchive          bool
}

type SourceRegistry struct {
	ordered  []SourceInfo
	bySource map[Source]SourceInfo
}

// DefaultSourceRegistry provides a fallback registry for zero-value UI models
// and isolated unit tests. Production runtime wiring constructs the registry
// directly from registered source adapters in cmd/agent-sessions.
func DefaultSourceRegistry() SourceRegistry {
	return NewSourceRegistry([]SourceInfo{
		{
			Source:                   SourceClaude,
			Label:                    "Claude",
			Badge:                    "C",
			DefaultPermissionMode:    PermissionModeFast,
			SupportsSafeResumeAction: true,
			SupportsFork:             true,
			SupportsArchive:          true,
		},
		{
			Source:                   SourceCodex,
			Label:                    "Codex",
			Badge:                    "X",
			DefaultPermissionMode:    PermissionModeSafe,
			SupportsSafeResumeAction: false,
			SupportsFork:             true,
			SupportsArchive:          true,
		},
		{
			Source:                   SourceOpenCode,
			Label:                    "OpenCode",
			Badge:                    "O",
			DefaultPermissionMode:    PermissionModeSafe,
			SupportsSafeResumeAction: false,
			SupportsFork:             true,
			SupportsArchive:          false,
		},
		{
			Source:                SourceAntigravityIDE,
			Label:                 "Antigravity IDE",
			Badge:                 "I",
			ResumeLabel:           "Open IDE",
			DefaultPermissionMode: PermissionModeSafe,
		},
		{
			Source:                SourceAntigravityCLI,
			Label:                 "Antigravity CLI",
			Badge:                 "A",
			DefaultPermissionMode: PermissionModeSafe,
		},
	})
}

func NewSourceRegistry(infos []SourceInfo) SourceRegistry {
	r := SourceRegistry{
		ordered:  make([]SourceInfo, 0, len(infos)),
		bySource: make(map[Source]SourceInfo, len(infos)),
	}
	for _, info := range infos {
		info = normalizeSourceInfo(info)
		r.ordered = append(r.ordered, info)
		r.bySource[info.Source] = info
	}
	return r
}

func (r SourceRegistry) All() []SourceInfo {
	r = r.withDefault()
	out := make([]SourceInfo, len(r.ordered))
	copy(out, r.ordered)
	return out
}

func (r SourceRegistry) Info(source Source) SourceInfo {
	r = r.withDefault()
	if info, ok := r.bySource[source]; ok {
		return info
	}
	return fallbackSourceInfo(source)
}

func (r SourceRegistry) DefaultPermissionMode(source Source) PermissionMode {
	return r.Info(source).DefaultPermissionMode
}

func (r SourceRegistry) SupportsSafeResumeAction(source Source) bool {
	return r.Info(source).SupportsSafeResumeAction
}

func (r SourceRegistry) SupportsFork(source Source) bool {
	return r.Info(source).SupportsFork
}

func (r SourceRegistry) SupportsArchive(source Source) bool {
	return r.Info(source).SupportsArchive
}

func (r SourceRegistry) withDefault() SourceRegistry {
	if len(r.ordered) > 0 || len(r.bySource) > 0 {
		return r
	}
	return DefaultSourceRegistry()
}

func normalizeSourceInfo(info SourceInfo) SourceInfo {
	fallback := fallbackSourceInfo(info.Source)
	if strings.TrimSpace(info.Label) == "" {
		info.Label = fallback.Label
	}
	if strings.TrimSpace(info.Badge) == "" {
		info.Badge = fallback.Badge
	}
	if info.ResumeLabel == "" {
		info.ResumeLabel = fallback.ResumeLabel
	}
	if info.DefaultPermissionMode == "" {
		info.DefaultPermissionMode = PermissionModeSafe
	}
	return info
}

func fallbackSourceInfo(source Source) SourceInfo {
	label := string(source)
	if strings.TrimSpace(label) == "" {
		label = "unknown"
	}
	return SourceInfo{
		Source:                source,
		Label:                 label,
		Badge:                 "?",
		ResumeLabel:           "Resume",
		DefaultPermissionMode: PermissionModeSafe,
	}
}
