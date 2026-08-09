package local

import (
	"context"
	"errors"
	"io"
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
