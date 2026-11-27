package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinguo998/agent-sessions/internal/domain"
	"github.com/jinguo998/agent-sessions/internal/storage"
)

func TestMetadataCacheValidatesFingerprint(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatal(err)
	}

	store := NewAt(filepath.Join(dir, "cache.json"))
	fp := storage.CacheFingerprint{Size: info.Size(), ModTime: info.ModTime(), MetadataKey: "v1"}
	want := storage.CachedSession{Session: domain.Session{ID: "cached", MsgCount: 1, FilePath: sessionPath}, SearchText: "cached text"}
	store.Put(sessionPath, fp, want)
	store.Save()

	got, ok := store.Get(sessionPath, fp)
	if !ok {
		t.Fatal("cache should hit for unchanged fingerprint after Save()")
	}
	if got.Session.ID != want.Session.ID || got.SearchText != want.SearchText {
		t.Fatalf("cache session = %#v, want %#v", got, want)
	}

	if _, ok := store.Get(sessionPath, storage.CacheFingerprint{Size: info.Size(), ModTime: info.ModTime(), MetadataKey: "v2"}); ok {
		t.Fatal("cache should miss when MetadataKey changes")
	}

	if err := os.WriteFile(sessionPath, []byte("two changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	changedInfo, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get(sessionPath, storage.CacheFingerprint{Size: changedInfo.Size(), ModTime: changedInfo.ModTime(), MetadataKey: "v1"}); ok {
		t.Fatal("cache should miss after file size/mtime changes")
	}
}

func TestMetadataCachePromotesAndPrunesAcrossEpochs(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "metadata-cache.json")
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

	fpA := storage.CacheFingerprint{Size: 10, ModTime: now, MetadataKey: "a"}
	fpB := storage.CacheFingerprint{Size: 20, ModTime: now, MetadataKey: "b"}
	sessA := storage.CachedSession{Session: domain.Session{ID: "a", MsgCount: 1}, SearchText: "alpha"}
	sessB := storage.CachedSession{Session: domain.Session{ID: "b", MsgCount: 2}, SearchText: "bravo"}

	// Scan epoch 1: populate both a and b, then Save (simulating startup scan).
	store := NewAt(cachePath)
	store.Put("/a.jsonl", fpA, sessA)
	store.Put("/b.jsonl", fpB, sessB)
	store.Save()

	// Scan epoch 2 (simulating Ctrl+R refresh in same process): only a still exists.
	if got, ok := store.Get("/a.jsonl", fpA); !ok || got.Session.ID != "a" {
		t.Fatalf("epoch 2 Get(a) = %#v, %v; want hit for a", got, ok)
	}
	store.Save()

	// Scan epoch 3: a remains warm in memory & on disk; b was pruned.
	if got, ok := store.Get("/a.jsonl", fpA); !ok || got.Session.ID != "a" {
		t.Fatalf("epoch 3 Get(a) = %#v, %v; want hit for a", got, ok)
	}
	if _, ok := store.Get("/b.jsonl", fpB); ok {
		t.Fatal("epoch 3 Get(b) should miss after pruning in epoch 2")
	}

	reloaded := NewAt(cachePath)
	if _, ok := reloaded.Get("/a.jsonl", fpA); !ok {
		t.Fatal("reloaded cache should retain a")
	}
	if _, ok := reloaded.Get("/b.jsonl", fpB); ok {
		t.Fatal("reloaded cache should not retain pruned b")
	}
}
