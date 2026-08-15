package video

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/TheZupZup/Rivo/services/api/internal/storage"
)

var (
	ErrEmptyVideo      = errors.New("video file is empty")
	ErrMissingVideo    = errors.New("video file is required")
	ErrGenerateVideoID = errors.New("generate video id")
	ErrStoreVideo      = errors.New("store video source")
)

type UploadRequest struct {
	FileName    string
	ContentType string
	Source      io.Reader
}

type UploadedVideo struct {
	ID          string `json:"id"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Status      string `json:"status"`
}

type UploadService struct {
	store storage.VideoStore
	newID func() (string, error)
}

func NewUploadService(store storage.VideoStore) UploadService {
	return UploadService{
		store: store,
		newID: newVideoID,
	}
}

func (service UploadService) Upload(ctx context.Context, request UploadRequest) (UploadedVideo, error) {
	if request.Source == nil {
		return UploadedVideo{}, ErrMissingVideo
	}

	firstByte := make([]byte, 1)
	readCount, readErr := request.Source.Read(firstByte)
	if errors.Is(readErr, io.EOF) && readCount == 0 {
		return UploadedVideo{}, ErrEmptyVideo
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return UploadedVideo{}, fmt.Errorf("read video source: %w", readErr)
	}

	videoID, err := service.newID()
	if err != nil {
		return UploadedVideo{}, fmt.Errorf("%w: %w", ErrGenerateVideoID, err)
	}

	storageKey := filepath.ToSlash(filepath.Join("videos", videoID, "source"))
	source := io.MultiReader(bytes.NewReader(firstByte[:readCount]), request.Source)
	if err := service.store.Save(ctx, storageKey, source); err != nil {
		return UploadedVideo{}, fmt.Errorf("%w: %w", ErrStoreVideo, err)
	}

	return UploadedVideo{
		ID:          videoID,
		FileName:    filepath.Base(request.FileName),
		ContentType: request.ContentType,
		Status:      "stored",
	}, nil
}

func newVideoID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
