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
	ErrEmptyVideo           = errors.New("video file is empty")
	ErrMissingVideo         = errors.New("video file is required")
	ErrUnsupportedContainer = errors.New("file is not a recognised video container")
	ErrGenerateVideoID      = errors.New("generate video id")
	ErrStoreVideo           = errors.New("store video source")
)

type UploadRequest struct {
	FileName string
	// PublisherUserID is the authenticated actor. Uploads are attributable because
	// moderation later has to name a creator, and an unattributable upload cannot be
	// judged under any ruleset.
	PublisherUserID string
	Source          io.Reader
}

type UploadedVideo struct {
	ID string `json:"id"`
	// FileName is the client's name, kept for display only and reduced to its base
	// so a crafted name cannot escape a directory when it is rendered or reused.
	FileName string `json:"fileName"`
	// ContentType is what the bytes were detected to be, not what the client claimed.
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

	header := make([]byte, headerBytes)
	readCount, err := io.ReadFull(request.Source, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return UploadedVideo{}, fmt.Errorf("read video source: %w", err)
	}
	header = header[:readCount]

	if readCount == 0 {
		return UploadedVideo{}, ErrEmptyVideo
	}

	contentType, recognised := DetectContainer(header)
	if !recognised {
		return UploadedVideo{}, ErrUnsupportedContainer
	}

	videoID, err := service.newID()
	if err != nil {
		return UploadedVideo{}, fmt.Errorf("%w: %w", ErrGenerateVideoID, err)
	}

	storageKey := filepath.ToSlash(filepath.Join("videos", videoID, "source"))
	source := io.MultiReader(bytes.NewReader(header), request.Source)
	if err := service.store.Save(ctx, storageKey, source); err != nil {
		return UploadedVideo{}, fmt.Errorf("%w: %w", ErrStoreVideo, err)
	}

	return UploadedVideo{
		ID:          videoID,
		FileName:    filepath.Base(request.FileName),
		ContentType: contentType,
		Status:      "stored",
	}, nil
}

func newVideoID() (string, error) {
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return "", err
	}

	return hex.EncodeToString(identifier), nil
}
