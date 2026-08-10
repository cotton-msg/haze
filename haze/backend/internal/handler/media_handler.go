package handler

import (
	"bytes"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type MediaHandler struct {
	svc *service.MediaService
}

func NewMediaHandler(svc *service.MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	result, err := h.svc.Upload(file, header, claims.Role == "premium")
	if err != nil {
		utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, err.Error())
		return
	}

	utils.SuccessResponse(w, result)
}

func (h *MediaHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	f, err := h.svc.GetFile(id)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "file not found")
		return
	}

	utils.SuccessResponse(w, f)
}

// Presign возвращает временную ссылку на файл (по умолчанию 15 минут).
func (h *MediaHandler) Presign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ttl := 15 * time.Minute
	if v := r.URL.Query().Get("expires"); v != "" {
		if secs, err := time.ParseDuration(v); err == nil && secs > 0 && secs <= 24*time.Hour {
			ttl = secs
		}
	}

	url, err := h.svc.PresignURL(id, ttl)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "file not found")
		return
	}

	utils.SuccessResponse(w, map[string]string{"url": url, "expires_in": ttl.String()})
}

func (h *MediaHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	rawPath := chi.URLParam(r, "*")
	if rawPath == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "missing path")
		return
	}

	// Отклоняем ".." до очистки пути (path.Clean сам нейтрализует его, из-за
	// чего пост-проверка никогда не сработает).
	for _, seg := range strings.Split(rawPath, "/") {
		if seg == ".." {
			utils.ErrorResponse(w, http.StatusBadRequest, "invalid path")
			return
		}
	}
	clean := path.Clean("/" + rawPath)

	rc, err := h.svc.GetFileByPath(clean)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "file not found")
		return
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	ext := filepath.Ext(clean)
	mime := mimeByExt(ext)
	if mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeContent(w, r, clean, time.Now(), bytes.NewReader(data))
}

func mimeByExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
