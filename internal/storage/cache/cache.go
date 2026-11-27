package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/jinguo998/claude-sessions/internal/storage"
)

const metadataCacheVersion = 5

type metadataCacheFile struct {
	Version int                   `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
}

type cacheEntry struct {
	Size        int64                 `json:"size"`
	ModTime     int64                 `json:"mod_time"`
	MetadataKey string                `json:"metadata_key,omitempty"`
	Session     storage.CachedSession `json:"session"`
}

type Store struct {
	path    string
	entries map[string]cacheEntry
	updated map[string]cacheEntry
	mu      sync.Mutex
}

func New() *Store {
	path, err := metadataCachePath()
	if err != nil {
		return &Store{entries: map[string]cacheEntry{}, updated: map[string]cacheEntry{}}
	}
	return NewAt(path)
}

func NewAt(path string) *Store {
	c := &Store{
		path:    path,
		entries: map[string]cacheEntry{},
		updated: map[string]cacheEntry{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var file metadataCacheFile
	if json.Unmarshal(data, &file) != nil || file.Version != metadataCacheVersion {
		return c
	}
	c.entries = file.Entries
	if c.entries == nil {
		c.entries = map[string]cacheEntry{}
	}
	return c
}

func metadataCachePath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "claude-sessions", "metadata-cache.json"), nil
}

func (c *Store) Get(key string, fp storage.CacheFingerprint) (storage.CachedSession, bool) {
	if c == nil {
		return storage.CachedSession{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok || entry.Size != fp.Size || entry.ModTime != fp.ModTime.UnixNano() || entry.MetadataKey != fp.MetadataKey {
		return storage.CachedSession{}, false
	}
	c.updated[key] = entry
	return entry.Session, true
}

func (c *Store) Put(key string, fp storage.CacheFingerprint, sess storage.CachedSession) {
	if c == nil {
		return
	}
	entry := cacheEntry{
		Size:        fp.Size,
		ModTime:     fp.ModTime.UnixNano(),
		MetadataKey: fp.MetadataKey,
		Session:     sess,
	}
	c.mu.Lock()
	c.updated[key] = entry
	c.mu.Unlock()
}

// Save promotes the current scan epoch into the active lookup map, starts a
// fresh epoch buffer for subsequent scans (e.g. Ctrl+R refresh), and persists
// the promoted snapshot atomically when backed by disk.
func (c *Store) Save() {
	if c == nil {
		return
	}
	c.mu.Lock()
	snapshot := c.updated
	c.entries = snapshot
	c.updated = make(map[string]cacheEntry, len(snapshot))
	path := c.path
	c.mu.Unlock()

	if path == "" {
		return
	}
	file := metadataCacheFile{
		Version: metadataCacheVersion,
		Entries: snapshot,
	}
	data, err := json.Marshal(file)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
