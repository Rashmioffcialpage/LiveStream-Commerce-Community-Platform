package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"stream-service/internal/db"
)

const maxRecordingUploadBytes = 200 << 20 // 200MB -- generous for a demo, not a real VOD limit

// UploadRecording lets a channel owner attach a recording to their own
// ended stream. Requires status == "ended" rather than allowing it any
// time, since a recording is the artifact of a stream that already
// happened -- uploading one for a still-live or not-yet-started stream
// isn't a state that makes sense to represent.
func (h *Handler) UploadRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.requireStreamOwnership(w, r, id) {
		return
	}

	stream, err := h.DB.GetStream(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch stream")
		return
	}
	if stream.Status != "ended" {
		writeError(w, http.StatusConflict, "can only upload a recording for an ended stream")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRecordingUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or malformed multipart form (max 200MB)")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, `expected a multipart file field named "file"`)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read upload")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key := fmt.Sprintf("%s/%s", id, header.Filename)

	url, err := h.Storage.Upload(r.Context(), key, body, contentType)
	if err != nil {
		slog.Error("upload recording", "err", err)
		writeError(w, http.StatusBadGateway, "could not store recording")
		return
	}

	updated, err := h.DB.SetRecordingURL(r.Context(), id, url)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save recording url")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
