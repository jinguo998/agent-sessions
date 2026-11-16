package opencode

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type dbRunner interface {
	DBPath(context.Context) (string, error)
	Query(context.Context, string, string, any) error
}

type sqliteRunner struct{}

func (sqliteRunner) DBPath(ctx context.Context) (string, error) {
	_ = ctx
	return resolveDBPath()
}

func (sqliteRunner) Query(ctx context.Context, dbPath, query string, dest any) error {
	absolute, err := filepath.Abs(dbPath)
	if err != nil {
		return err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute), RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("opencode database query: %w", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([]any, len(columns))
	targets := make([]any, len(columns))
	for i := range values {
		targets[i] = &values[i]
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return err
		}
		row := make(map[string]any, len(columns))
		for i, name := range columns {
			row[name] = values[i]
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func resolveDBPath() (string, error) {
	dataDir, err := opencodeDataDir()
	if err != nil {
		return "", err
	}

	if env := strings.TrimSpace(os.Getenv("OPENCODE_DB")); env != "" {
		switch {
		case env == ":memory:":
			return "", fmt.Errorf("OPENCODE_DB=:memory: cannot be scanned")
		case filepath.IsAbs(env):
			return existingDBPath(env)
		default:
			return existingDBPath(filepath.Join(dataDir, env))
		}
	}

	candidates := []string{filepath.Join(dataDir, "opencode.db")}
	channelMatches, _ := filepath.Glob(filepath.Join(dataDir, "opencode-*.db"))
	sort.Strings(channelMatches)
	candidates = append(candidates, channelMatches...)
	for _, candidate := range candidates {
		if path, err := existingDBPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("opencode database not found under %s", dataDir)
}

func existingDBPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("opencode database path is a directory: %s", path)
	}
	return path, nil
}

func opencodeDataDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("OPENCODE_DATA_DIR")); dir != "" {
		return dir, nil
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "opencode"), nil
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
