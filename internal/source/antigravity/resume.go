package antigravity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/jinguo998/claude-sessions/internal/domain"
	"github.com/jinguo998/claude-sessions/internal/source"
)

func (a Adapter) Descriptor() source.Descriptor {
	if a.kind == domain.SourceAntigravityCLI {
		return source.Descriptor{
			Source:                domain.SourceAntigravityCLI,
			Label:                 "Antigravity CLI",
			Badge:                 "A",
			DefaultPermissionMode: domain.PermissionModeSafe,
		}
	}
	return source.Descriptor{
		Source:                domain.SourceAntigravityIDE,
		Label:                 "Antigravity IDE",
		Badge:                 "I",
		ResumeLabel:           "Open IDE",
		DefaultPermissionMode: domain.PermissionModeSafe,
	}
}

func (a Adapter) PlanResume(ctx context.Context, target domain.ResumeTarget) (domain.ResumePlan, error) {
	s := target.Session
	plan := domain.ResumePlan{WorkingDir: s.ProjectPath, HandoffDir: s.ProjectPath}
	switch target.Action {
	case domain.ResumeActionCd:
		if s.ProjectPath == "" {
			return domain.ResumePlan{}, fmt.Errorf("Antigravity recorded no local workspace for %s", s.ID)
		}
		plan.CdOnly = true
		return plan, nil
	case domain.ResumeActionResume:
	default:
		return domain.ResumePlan{}, fmt.Errorf("%s %s: %w", a.kind, target.Action, source.ErrUnsupportedCapability)
	}
	if s.ID == "" {
		return domain.ResumePlan{}, fmt.Errorf("missing Antigravity conversation ID")
	}
	// Conversations can outlive a temporary project or a deleted worktree.
	// Both clients can open history from the current directory in that case.
	if s.ProjectPath != "" {
		if info, err := os.Stat(s.ProjectPath); err != nil || !info.IsDir() {
			plan.WorkingDir, plan.HandoffDir = "", ""
		}
	}
	if a.kind == domain.SourceAntigravityCLI {
		plan.Executable = "agy"
		plan.Args = []string{"agy", "--conversation", s.ID}
		plan.Message = fmt.Sprintf("Resuming Antigravity CLI conversation %s", s.ID)
		return plan, nil
	}
	launcher, app := "antigravity-ide", "Antigravity IDE"
	if s.Client == "antigravity" {
		launcher, app = "antigravity", "Antigravity"
	}
	plan.Executable = launcher
	plan.Args = []string{launcher}
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath(launcher); err != nil {
			plan.Executable = "open"
			plan.Args = []string{"open", "-a", app}
		}
	}
	if plan.WorkingDir != "" {
		plan.Args = append(plan.Args, plan.WorkingDir)
	}
	plan.Message = fmt.Sprintf("Opening %s. Select conversation %s in history.", app, s.ID)
	if s.Title != "" {
		plan.Message = fmt.Sprintf("Opening %s. Select %q (%s) in history.", app, s.Title, s.ID)
	}
	return plan, nil
}
