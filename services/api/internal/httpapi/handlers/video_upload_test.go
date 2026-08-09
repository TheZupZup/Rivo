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

	platformvideo "github.com/TheZupZup/Rivo/services/api/internal/platform/video"
	"github.com/TheZupZup/Rivo/services/api/internal/storage/local"
)

func TestVideoUploadStoresMultipartFile(t *testing.T) {
	storageRoot := t.TempDir()
	store, err := local.New(storageRoot)
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}

	handler := NewVideoUploadHandler(platformvideo.NewUploadService(store), 1024*1024)
	body, contentType := multipartVideoBody(t, "video", "clip.mp4", []byte("video bytes"))
	request := httptest.NewRequest(http.MethodPost, "/api/videos", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	handler.Upload(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload struct {
		ID     string `json:"id"`
		Status string `json:"status"`
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

	storedPath := filepath.Join(storageRoot, "videos", payload.ID, "source")
	storedBytes, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored upload: %v", err)
	}
	if string(storedBytes) != "video bytes" {
		t.Fatalf("unexpected stored content %q", string(storedBytes))
	}
}

func TestVideoUploadRejectsMissingVideoPart(t *testing.T) {
	store, err := local.New(t.TempDir())
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}

	handler := NewVideoUploadHandler(platformvideo.NewUploadService(store), 1024*1024)
	body, contentType := multipartVideoBody(t, "other", "clip.mp4", []byte("video bytes"))
	request := httptest.NewRequest(http.MethodPost, "/api/videos", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	handler.Upload(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
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
