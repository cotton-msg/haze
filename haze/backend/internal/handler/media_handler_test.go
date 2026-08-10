package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// hTestFileRepo — in-memory реализация service.FileRepository.
type hTestFileRepo struct {
	files map[string]*models.File
}

func newHTestFileRepo() *hTestFileRepo {
	return &hTestFileRepo{files: map[string]*models.File{}}
}

func (m *hTestFileRepo) Create(file *models.File) error {
	if file.ID == "" {
		file.ID = "file-1"
	}
	m.files[file.ID] = file
	return nil
}

func (m *hTestFileRepo) FindByID(id string) (*models.File, error) {
	if f, ok := m.files[id]; ok {
		return f, nil
	}
	return &models.File{}, nil
}

func (m *hTestFileRepo) FindByMessageID(messageID string) ([]*models.File, error) {
	return nil, nil
}

func (m *hTestFileRepo) Delete(id string) error {
	delete(m.files, id)
	return nil
}

// hTestStorage — in-memory реализация service.StorageBackend.
type hTestStorage struct {
	data map[string][]byte
}

func newHTestStorage() *hTestStorage {
	return &hTestStorage{data: map[string][]byte{}}
}

func (s *hTestStorage) Save(path string, reader io.Reader) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.data[path] = b
	return nil
}

func (s *hTestStorage) Get(path string) (io.ReadCloser, error) {
	b, ok := s.data[path]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (s *hTestStorage) Delete(path string) error {
	delete(s.data, path)
	return nil
}

func (s *hTestStorage) URL(path string) string {
	return "http://media.test/" + path
}

func (s *hTestStorage) PresignedURL(path string, expires time.Duration) (string, error) {
	return "http://media.test/" + path, nil
}

func newMediaRouter(t *testing.T, fileRepo service.FileRepository, storage service.StorageBackend) http.Handler {
	t.Helper()
	svc := service.NewMediaService(fileRepo, storage, 10<<20, 100<<20)
	h := NewMediaHandler(svc)
	r := chi.NewRouter()
	r.Post("/upload", h.Upload)
	r.Get("/files/{id}", h.GetFile)
	r.Get("/serve/*", h.ServeFile)
	return r
}

func multipartRequest(t *testing.T, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	fw.Write([]byte(content))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestMediaHandlerUpload(t *testing.T) {
	router := newMediaRouter(t, newHTestFileRepo(), newHTestStorage())

	req := multipartRequest(t, "photo.png", "imagedata")
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestMediaHandlerUploadTooLarge(t *testing.T) {
	router := newMediaRouter(t, newHTestFileRepo(), newHTestStorage())

	req := multipartRequest(t, "big.bin", strings.Repeat("x", 20<<20))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

func TestMediaHandlerUploadMissingFile(t *testing.T) {
	router := newMediaRouter(t, newHTestFileRepo(), newHTestStorage())

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = withClaims(req, "user-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMediaHandlerGetFile(t *testing.T) {
	fileRepo := newHTestFileRepo()
	fileRepo.Create(&models.File{ID: "file-1", URL: "http://media.test/x.png", MimeType: "image/png"})

	router := newMediaRouter(t, fileRepo, newHTestStorage())

	req := httptest.NewRequest(http.MethodGet, "/files/file-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestMediaHandlerServeFile(t *testing.T) {
	storage := newHTestStorage()
	storage.data["2026/01/01/pic.png"] = []byte("pngdata")

	router := newMediaRouter(t, newHTestFileRepo(), storage)

	req := httptest.NewRequest(http.MethodGet, "/serve/2026/01/01/pic.png", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected image/png content type, got %q", ct)
	}
}

func TestMediaHandlerServeFileMissingPath(t *testing.T) {
	router := newMediaRouter(t, newHTestFileRepo(), newHTestStorage())

	req := httptest.NewRequest(http.MethodGet, "/serve/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMediaHandlerServeFileTraversal(t *testing.T) {
	router := newMediaRouter(t, newHTestFileRepo(), newHTestStorage())

	req := httptest.NewRequest(http.MethodGet, "/serve/../../etc/passwd", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d", w.Code)
	}
}

func TestMediaHandlerServeFileNotFound(t *testing.T) {
	router := newMediaRouter(t, newHTestFileRepo(), newHTestStorage())

	req := httptest.NewRequest(http.MethodGet, "/serve/missing.png", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

var _ = context.Background
