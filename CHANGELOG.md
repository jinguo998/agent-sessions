# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- MIT `LICENSE` file and release archive packaging of `LICENSE`.
- Repository engineering files: `SECURITY.md`, `CHANGELOG.md`, `.editorconfig`, `.golangci.yml`, `.github/dependabot.yml`, `.github/release.yml`, issue forms, and pull request template.
- Formatting (`gofmt`) and `golangci-lint` checks in CI, plus `fmt`, `fmt-check`, `test-race`, `cover`, and `check` targets in `Makefile`.

### Changed

- Unified source capability registration via `source.Describer` in the composition root and moved terminal color palettes out of `internal/app/model` into `internal/ui/shared`.
- Upgraded `storage.MetadataCache` to validate `CacheFingerprint` (`Size`, `ModTime`, `MetadataKey`) and promote scan epochs on `Save()` so `Ctrl+R` refreshes reuse in-memory cache.
- Eliminated lossy `tui.Result` handoff by returning `domain.ResumeTarget` directly, embedded `domain.Session` in `model.Session`, and removed terminal display-width dependencies from `internal/app/stats`.
- Extracted shared UI formatting/theme helpers into `internal/ui/shared` and moved runtime tracing to `internal/platform/trace` behind `ports.Tracer`.
- Surfaced partial scan warnings in the TUI status bar and isolated shell `cs` directory handoff with per-invocation `mktemp` files (`CLAUDE_SESSIONS_CD_FILE`).

## [0.3.0] - 2026-11-26

### Added

- Support for Antigravity IDE and Antigravity CLI session histories under `~/.gemini/` (including desktop app and legacy IDE profiles).
- Separate Antigravity IDE and CLI source filters in the TUI.
- Antigravity transcript parsing and SQLite summary index fallback for previews.
- Direct CLI conversation resume (`agy --conversation <id>`) and IDE workspace launching.

## [0.2.0] - 2026-11-18

### Added

- Compatibility with updated Claude Code, Codex CLI, and OpenCode session formats.
- Claude subagent transcript support in verbose preview.
- OpenCode schema v2 support.
- Homebrew tap installation instructions in `README.md`.

## [0.1.0] - 2026-07-29

### Added

- Initial public release of `claude-sessions`.
- Full-screen Bubble Tea TUI dashboard and inline fuzzy picker (`claude-sessions pick`) for Claude Code, Codex CLI, and OpenCode sessions.
- Live split preview with Markdown rendering, search, stats view, and trash archive/restore for Claude and Codex sessions.
- Zsh and Bash shell integration (`claude-sessions init`) with `cs` alias and `Ctrl+S` fuzzy picker binding.
- Automated Linux and macOS (`amd64` / `arm64`) release archives with SHA-256 checksums via GitHub Actions.

[Unreleased]: https://github.com/jinguo998/claude-sessions/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/jinguo998/claude-sessions/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/jinguo998/claude-sessions/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/jinguo998/claude-sessions/releases/tag/v0.1.0
