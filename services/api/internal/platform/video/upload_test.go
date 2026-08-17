package video

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// sampleMP4 is a minimal ISO base media header: a box length followed by "ftyp".
var sampleMP4 = []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2avc1mp41")

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

	// A payload larger than the sniffed header proves the buffered prefix is
	// replayed into storage rather than swallowed by detection.
	content := append(append([]byte{}, sampleMP4...), bytes.Repeat([]byte("m"), 4096)...)

	result, err := service.Upload(context.Background(), UploadRequest{
		FileName:        "clip.mp4",
		PublisherUserID: "user-1",
		Source:          bytes.NewReader(content),
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
	if store.content != string(content) {
		t.Fatalf("expected every source byte to be stored, got %d of %d bytes", len(store.content), len(content))
	}
}

func TestUploadRecordsDetectedContentTypeNotTheFileName(t *testing.T) {
	service := UploadService{
		store: &recordingStore{},
		newID: func() (string, error) { return "video-123", nil },
	}

	result, err := service.Upload(context.Background(), UploadRequest{
		// A WebM payload announced as ".mp4": the bytes decide, not the name.
		FileName: "clip.mp4",
		Source:   bytes.NewReader(append([]byte{0x1A, 0x45, 0xDF, 0xA3}, bytes.Repeat([]byte{0}, 32)...)),
	})
	if err != nil {
		t.Fatalf("upload video: %v", err)
	}

	if result.ContentType != "video/webm" {
		t.Fatalf("expected detected content type video/webm, got %q", result.ContentType)
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

func TestUploadRejectsNonVideoPayload(t *testing.T) {
	store := &recordingStore{}
	service := UploadService{
		store: store,
		newID: func() (string, error) { return "unused", nil },
	}

	_, err := service.Upload(context.Background(), UploadRequest{
		FileName: "payload.mp4",
		Source:   strings.NewReader("#!/bin/sh\necho this is not a video\n"),
	})

	if !errors.Is(err, ErrUnsupportedContainer) {
		t.Fatalf("expected ErrUnsupportedContainer, got %v", err)
	}
	if store.key != "" {
		t.Fatalf("expected nothing to be written, got key %q", store.key)
	}
}

func TestUploadRejectsMissingSource(t *testing.T) {
	service := NewUploadService(&recordingStore{})

	if _, err := service.Upload(context.Background(), UploadRequest{FileName: "clip.mp4"}); !errors.Is(err, ErrMissingVideo) {
		t.Fatalf("expected ErrMissingVideo, got %v", err)
	}
}
