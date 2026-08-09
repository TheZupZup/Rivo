package video

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type recordingStore struct {
	key     string
	content string
	err     error
}

func (store *recordingStore) Save(_ context.Context, key string, source io.Reader) error {
	if store.err != nil {
		return store.err
	}

	content, err := io.ReadAll(source)
	if err != nil {
		return err
	}

	store.key = key
	store.content = string(content)
	return nil
}

func (store *recordingStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (store *recordingStore) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

func TestUploadStoresSourceUnderGeneratedVideoID(t *testing.T) {
	store := &recordingStore{}
	service := UploadService{
		store: store,
		newID: func() (string, error) {
			return "video-123", nil
		},
	}

	result, err := service.Upload(context.Background(), UploadRequest{
		FileName:    "clip.mp4",
		ContentType: "video/mp4",
		Source:      strings.NewReader("video bytes"),
	})
	if err != nil {
		t.Fatalf("upload video: %v", err)
	}

	if result.ID != "video-123" {
		t.Fatalf("expected video id %q, got %q", "video-123", result.ID)
	}
	if result.Status != "stored" {
		t.Fatalf("expected stored status, got %q", result.Status)
	}
	if store.key != "videos/video-123/source" {
		t.Fatalf("expected deterministic storage key, got %q", store.key)
	}
	if store.content != "video bytes" {
		t.Fatalf("expected source bytes to be stored, got %q", store.content)
	}
}

func TestUploadRejectsEmptyVideo(t *testing.T) {
	service := UploadService{
		store: &recordingStore{},
		newID: func() (string, error) {
			return "unused", nil
		},
	}

	_, err := service.Upload(context.Background(), UploadRequest{
		FileName: "empty.mp4",
		Source:   strings.NewReader(""),
	})

	if !errors.Is(err, ErrEmptyVideo) {
		t.Fatalf("expected ErrEmptyVideo, got %v", err)
	}
}
