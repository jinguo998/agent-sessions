package shell

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/jinguo998/claude-sessions/internal/domain"
)

const (
	// CdHandoffEnv carries a per-invocation temporary file path created by the
	// shell wrapper (cs / picker) so concurrent terminals never race on a shared
	// /tmp file.
	CdHandoffEnv = "CLAUDE_SESSIONS_CD_FILE"
	// CdHandoffFile is the legacy fallback path used when CdHandoffEnv is unset.
	CdHandoffFile = "/tmp/claude-sessions-cd"
)

func cdHandoffTarget() (string, os.FileMode) {
	if path := os.Getenv(CdHandoffEnv); path != "" {
		return path, 0o600
	}
	return CdHandoffFile, 0o644
}

func writeCdHandoff(dir string) error {
	path, mode := cdHandoffTarget()
	if err := os.WriteFile(path, []byte(dir), mode); err != nil {
		return err
	}
	if mode == 0o600 {
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func CdAndHandoff(dir string) error {
	fmt.Fprintf(os.Stderr, "\033[2m→ cd %s\033[0m\n", dir)
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("cd failed: %w", err)
	}
	return writeCdHandoff(dir)
}

func ExecPlan(plan domain.ResumePlan) error {
	if plan.WorkingDir != "" {
		if err := CdAndHandoff(plan.WorkingDir); err != nil {
			return err
		}
	}
	if plan.Message != "" {
		fmt.Fprintf(os.Stderr, "\033[2m→ %s\033[0m\n", plan.Message)
	}
	if plan.CdOnly {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/zsh"
		}
		shellPath, err := exec.LookPath(shell)
		if err != nil {
			return fmt.Errorf("shell not found: %w", err)
		}
		return syscall.Exec(shellPath, []string{shell}, os.Environ())
	}
	binPath, err := exec.LookPath(plan.Executable)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w", plan.Executable, err)
	}
	return syscall.Exec(binPath, plan.Args, os.Environ())
}
