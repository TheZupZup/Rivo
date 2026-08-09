package handlers

import (
	"errors"
	"io"
	"net/http"

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
	request.Body = http.MaxBytesReader(w, request.Body, handler.maxUploadBytes)

	multipartReader, err := request.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "request must use multipart/form-data")
		return
	}

	for {
		part, err := multipartReader.NextPart()
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "video file is required")
			return
		}
		if err != nil {
			if isRequestTooLarge(err) {
				writeError(w, http.StatusRequestEntityTooLarge, "video upload exceeds the current 1 GiB limit")
				return
			}

			writeError(w, http.StatusBadRequest, "could not read multipart upload")
			return
		}

		if part.FormName() != "video" || part.FileName() == "" {
			_ = part.Close()
			continue
		}

		result, err := handler.uploadService.Upload(request.Context(), platformvideo.UploadRequest{
			FileName:    part.FileName(),
			ContentType: part.Header.Get("Content-Type"),
			Source:      part,
		})
		_ = part.Close()
		if err != nil {
			handler.writeUploadError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
		return
	}
}

func (handler VideoUploadHandler) writeUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, platformvideo.ErrEmptyVideo), errors.Is(err, platformvideo.ErrMissingVideo):
		writeError(w, http.StatusBadRequest, err.Error())
	case isRequestTooLarge(err):
		writeError(w, http.StatusRequestEntityTooLarge, "video upload exceeds the current 1 GiB limit")
	default:
		writeError(w, http.StatusInternalServerError, "video upload failed")
	}
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}
