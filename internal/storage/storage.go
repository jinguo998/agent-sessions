package storage

import (
	"time"

	"github.com/jinguo998/claude-sessions/internal/domain"
)

// CacheFingerprint uniquely validates whether a cached session entry is still fresh.
// It unifies file-backed (Size, ModTime) and index/DB-backed (MetadataKey) invalidation.
type CacheFingerprint struct {
	Size        int64
	ModTime     time.Time
	MetadataKey string
}

type CachedSession struct {
	Session    domain.Session
	SearchText string
}

type MetadataCache interface {
	Get(key string, fp CacheFingerprint) (CachedSession, bool)
	Put(key string, fp CacheFingerprint, sess CachedSession)
	Save()
}

type ArchivedSessionMetadata struct {
	ID               string        `json:"id"`
	Source           domain.Source `json:"source"`
	ArchivedAt       string        `json:"archived_at"`
	OriginalFilePath string        `json:"original_file_path"`
	OriginalSideDir  string        `json:"original_side_dir,omitempty"`
	ProjectPath      string        `json:"project_path,omitempty"`
	Title            string        `json:"title,omitempty"`
	LegacyThreadName string        `json:"thread_name,omitempty"`
}

type ArchivedSession struct {
	ArchiveDir  string
	Metadata    ArchivedSessionMetadata
	SessionFile string
	SideDir     string
}

type TrashStore interface {
	Archive(domain.Session, string) (string, error)
	List() ([]ArchivedSession, error)
	Restore(ArchivedSession) error
	Delete(ArchivedSession) error
	DeleteAll([]ArchivedSession) error
}
