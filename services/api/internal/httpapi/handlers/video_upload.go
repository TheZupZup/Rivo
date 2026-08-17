package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	platformvideo "github.com/TheZupZup/Rivo/services/api/internal/platform/video"
)

const defaultMaxUploadBytes int64 = 1024 * 1024 * 1024 // 1 GiB for the local prototype.

type VideoUploadHandler struct {
	uploadService  platformvideo.UploadService
	maxUploadBytes int64
}

func NewVideoUploadHandler(uploadService platformvideo.UploadService, maxUploadBytes int64) VideoUploadHandler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}

	return VideoUploadHandler{
		uploadService:  uploadService,
		maxUploadBytes: maxUploadBytes,
	}
}

func (handler VideoUploadHandler) Upload(w http.ResponseWriter, request *http.Request) {
	identity, _ := auth.IdentityFrom(request.Context())

	request.Body = http.MaxBytesReader(w, request.Body, handler.maxUploadBytes)

	multipartReader, err := request.MultipartReader()
	if err != nil {
		WriteError(w, http.StatusBadRequest, "request must use multipart/form-data")
		return
	}

	for {
		part, err := multipartReader.NextPart()
		if errors.Is(err, io.EOF) {
			WriteError(w, http.StatusBadRequest, "video file is required")
			return
		}
		if err != nil {
			if isRequestTooLarge(err) {
				handler.writeTooLarge(w)
				return
			}

			WriteError(w, http.StatusBadRequest, "could not read multipart upload")
			return
		}

		if part.FormName() != "video" || part.FileName() == "" {
			_ = part.Close()
			continue
		}

		result, err := handler.uploadService.Upload(request.Context(), platformvideo.UploadRequest{
			FileName:        part.FileName(),
			PublisherUserID: identity.UserID,
			Source:          part,
		})
		_ = part.Close()
		if err != nil {
			handler.writeUploadError(w, err)
			return
		}

		WriteJSON(w, http.StatusCreated, result)
		return
	}
}

func (handler VideoUploadHandler) writeUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, platformvideo.ErrEmptyVideo), errors.Is(err, platformvideo.ErrMissingVideo):
		WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, platformvideo.ErrUnsupportedContainer):
		WriteError(w, http.StatusUnsupportedMediaType, "upload must be an MP4, WebM, Matroska, AVI, MPEG, FLV or Ogg video")
	case isRequestTooLarge(err):
		handler.writeTooLarge(w)
	default:
		WriteError(w, http.StatusInternalServerError, "video upload failed")
	}
}

func (handler VideoUploadHandler) writeTooLarge(w http.ResponseWriter) {
	WriteError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf(
		"video upload exceeds the current %d byte limit", handler.maxUploadBytes,
	))
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}
