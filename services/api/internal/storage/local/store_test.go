package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSaveOpenAndDelete(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	ctx := context.Background()
	key := "channels/demo/video.mp4"
	if err := store.Save(ctx, key, strings.NewReader("video-data")); err != nil {
		t.Fatalf("save video: %v", err)
	}

	reader, err := store.Open(ctx, key)
	if err != nil {
		t.Fatalf("open video: %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read video: %v", err)
	}
	if string(content) != "video-data" {
		t.Fatalf("unexpected content %q", content)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete video: %v", err)
	}
}

func TestStoreDeleteLeavesNoEmptyDirectoriesBehind(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	ctx := context.Background()
	if err := store.Save(ctx, "videos/abc123/source", strings.NewReader("video-data")); err != nil {
		t.Fatalf("save video: %v", err)
	}
	if err := store.Delete(ctx, "videos/abc123/source"); err != nil {
		t.Fatalf("delete video: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "videos", "abc123")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected the per-video directory to be pruned, got %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("expected the storage root to survive pruning, got %v", err)
	}
}

func TestStoreDeleteKeepsDirectoriesThatStillHoldObjects(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	ctx := context.Background()
	if err := store.Save(ctx, "videos/abc123/source", strings.NewReader("video-data")); err != nil {
		t.Fatalf("save source: %v", err)
	}
	if err := store.Save(ctx, "videos/abc123/thumbnail", strings.NewReader("thumb-data")); err != nil {
		t.Fatalf("save thumbnail: %v", err)
	}

	if err := store.Delete(ctx, "videos/abc123/source"); err != nil {
		t.Fatalf("delete source: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "videos", "abc123", "thumbnail")); err != nil {
		t.Fatalf("expected the remaining object to survive, got %v", err)
	}
}

// A client that repeatedly aborts uploads must not be able to litter the storage
// root with empty directories until it runs out of inodes.
func TestStoreSaveCleansUpAfterAFailedCopy(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	truncated := errors.New("connection reset")
	err = store.Save(context.Background(), "videos/abc123/source", failingReader{err: truncated})
	if !errors.Is(err, truncated) {
		t.Fatalf("expected the copy error to surface, got %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "videos"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read storage root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no directories left behind, got %d", len(entries))
	}
}

type failingReader struct {
	err error
}

func (reader failingReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func TestStoreDeleteIsIdempotent(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if err := store.Delete(context.Background(), "videos/never-existed/source"); err != nil {
		t.Fatalf("expected deleting an absent object to succeed, got %v", err)
	}
}

func TestStoreRejectsPathTraversal(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	err = store.Save(context.Background(), "../escape", strings.NewReader("nope"))
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}
