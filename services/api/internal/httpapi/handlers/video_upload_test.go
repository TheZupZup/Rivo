package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	platformvideo "github.com/TheZupZup/Rivo/services/api/internal/platform/video"
	"github.com/TheZupZup/Rivo/services/api/internal/storage/local"
)

// sampleMP4 is a minimal ISO base media header: a box length followed by "ftyp".
var sampleMP4 = []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2avc1mp41")

func TestVideoUploadStoresMultipartFile(t *testing.T) {
	storageRoot := t.TempDir()
	store, err := local.New(storageRoot)
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}

	handler := NewVideoUploadHandler(platformvideo.NewUploadService(store), 1024*1024)
	body, contentType := multipartVideoBody(t, "video", "clip.mp4", sampleMP4)
	response := httptest.NewRecorder()

	handler.Upload(response, authenticatedUpload(body, contentType))

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		ContentType string `json:"contentType"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ID == "" {
		t.Fatal("expected generated video id")
	}
	if payload.Status != "stored" {
		t.Fatalf("expected stored status, got %q", payload.Status)
	}
	if payload.ContentType != "video/mp4" {
		t.Fatalf("expected the detected container, got %q", payload.ContentType)
	}

	storedPath := filepath.Join(storageRoot, "videos", payload.ID, "source")
	storedBytes, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored upload: %v", err)
	}
	if !bytes.Equal(storedBytes, sampleMP4) {
		t.Fatalf("unexpected stored content %q", storedBytes)
	}
}

func TestVideoUploadRejectsMissingVideoPart(t *testing.T) {
	handler := newTestUploadHandler(t, 1024*1024)
	body, contentType := multipartVideoBody(t, "other", "clip.mp4", sampleMP4)
	response := httptest.NewRecorder()

	handler.Upload(response, authenticatedUpload(body, contentType))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestVideoUploadRejectsAPayloadThatIsNotAVideo(t *testing.T) {
	handler := newTestUploadHandler(t, 1024*1024)
	body, contentType := multipartVideoBody(t, "video", "clip.mp4", []byte("#!/bin/sh\necho not a video\n"))
	response := httptest.NewRecorder()

	handler.Upload(response, authenticatedUpload(body, contentType))

	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnsupportedMediaType, response.Code, response.Body.String())
	}
}

func TestVideoUploadRejectsOversizedBody(t *testing.T) {
	handler := newTestUploadHandler(t, 32)
	body, contentType := multipartVideoBody(t, "video", "clip.mp4", bytes.Repeat([]byte("m"), 4096))
	response := httptest.NewRecorder()

	handler.Upload(response, authenticatedUpload(body, contentType))

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d: %s", http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	}
}

func TestVideoUploadRequiresMultipart(t *testing.T) {
	handler := newTestUploadHandler(t, 1024*1024)
	request := authenticatedUpload(bytes.NewBufferString(`{"video":"clip.mp4"}`), "application/json")
	response := httptest.NewRecorder()

	handler.Upload(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func newTestUploadHandler(t *testing.T, maxUploadBytes int64) VideoUploadHandler {
	t.Helper()

	store, err := local.New(t.TempDir())
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}

	return NewVideoUploadHandler(platformvideo.NewUploadService(store), maxUploadBytes)
}

func authenticatedUpload(body *bytes.Buffer, contentType string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/videos", body)
	request.Header.Set("Content-Type", contentType)

	return request.WithContext(auth.WithIdentity(request.Context(), auth.Identity{
		UserID: "user-1",
		Handle: "creator",
	}))
}

func multipartVideoBody(t *testing.T, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}
