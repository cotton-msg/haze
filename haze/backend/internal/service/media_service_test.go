package service

import (
	"bytes"
	"io"
	"mime/multipart"
	"testing"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type mockFileRepo struct {
	files   map[string]*models.File
	created []*models.File
	deleted []string
}

func newMockFileRepo() *mockFileRepo {
	return &mockFileRepo{files: map[string]*models.File{}}
}

func (m *mockFileRepo) Create(file *models.File) error {
	if file.ID == "" {
		file.ID = "file-1"
	}
	m.files[file.ID] = file
	m.created = append(m.created, file)
	return nil
}

func (m *mockFileRepo) FindByID(id string) (*models.File, error) {
	if f, ok := m.files[id]; ok {
		return f, nil
	}
	return nil, errNotFound
}

func (m *mockFileRepo) FindByMessageID(messageID string) ([]*models.File, error) {
	var out []*models.File
	for _, f := range m.files {
		if f.MessageID == messageID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *mockFileRepo) Delete(id string) error {
	m.deleted = append(m.deleted, id)
	delete(m.files, id)
	return nil
}

// memStorage — StorageBackend, пишущий в память.
type memStorage struct {
	data map[string][]byte
	url  string
}

func newMemStorage() *memStorage {
	return &memStorage{data: map[string][]byte{}, url: "http://media.test"}
}

func (m *memStorage) Save(path string, reader io.Reader) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.data[path] = b
	return nil
}

func (m *memStorage) Get(path string) (io.ReadCloser, error) {
	b, ok := m.data[path]
	if !ok {
		return nil, errNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStorage) Delete(path string) error {
	delete(m.data, path)
	return nil
}

func (m *memStorage) URL(path string) string {
	return m.url + "/" + path
}

func (m *memStorage) PresignedURL(path string, expires time.Duration) (string, error) {
	return m.url + "/" + path, nil
}

func newTestMediaService(repo *mockFileRepo, storage StorageBackend) *MediaService {
	return NewMediaService(repo, storage, 1024*1024, 10*1024*1024)
}

func multipartFile(t *testing.T, name, contentType, content string) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	reader := bytes.NewReader([]byte(content))
	return &mockMultipartFile{Reader: reader}, &multipart.FileHeader{Filename: name, Size: int64(len(content)), Header: map[string][]string{"Content-Type": {contentType}}}
}

// mockMultipartFile реализует multipart.File поверх bytes.Reader.
type mockMultipartFile struct {
	*bytes.Reader
}

func (m *mockMultipartFile) Close() error { return nil }

func TestUploadSmallFileSucceeds(t *testing.T) {
	repo := newMockFileRepo()
	storage := newMemStorage()
	svc := newTestMediaService(repo, storage)

	file, header := multipartFile(t, "photo.png", "image/png", "fake-image-bytes")
	defer file.Close()

	result, err := svc.Upload(file, header, false)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.URL == "" {
		t.Fatal("expected non-empty URL")
	}
	if result.Thumb == "" {
		t.Errorf("image should have thumbnail")
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 file record, got %d", len(repo.created))
	}
}

func TestUploadTooLargeRejected(t *testing.T) {
	repo := newMockFileRepo()
	storage := newMemStorage()
	svc := NewMediaService(repo, storage, 10, 100) // tiny limit

	file, header := multipartFile(t, "big.pdf", "application/pdf", "this content is way more than ten bytes")
	defer file.Close()

	_, err := svc.Upload(file, header, false)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestUploadPremiumGetsBiggerLimit(t *testing.T) {
	repo := newMockFileRepo()
	storage := newMemStorage()
	svc := NewMediaService(repo, storage, 10, 1000)

	file, header := multipartFile(t, "medium.bin", "application/octet-stream", "12345678901234567890") // 20 bytes
	defer file.Close()

	result, err := svc.Upload(file, header, true)
	if err != nil {
		t.Fatalf("premium upload should pass: %v", err)
	}
	if result.URL == "" {
		t.Fatal("expected URL")
	}
}

func TestGetFileByID(t *testing.T) {
	repo := newMockFileRepo()
	repo.files["f-1"] = &models.File{ID: "f-1", URL: "http://x/1.png", Name: "1.png"}
	storage := newMemStorage()
	svc := newTestMediaService(repo, storage)

	f, err := svc.GetFile("f-1")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if f.ID != "f-1" {
		t.Errorf("unexpected file: %s", f.ID)
	}
}

func TestDeleteFileRemovesRecordAndStorage(t *testing.T) {
	repo := newMockFileRepo()
	repo.files["f-1"] = &models.File{ID: "f-1", URL: "http://media.test/2026/01/01/id.png"}
	storage := newMemStorage()
	storage.data["2026/01/01/id.png"] = []byte("x")
	svc := newTestMediaService(repo, storage)

	if err := svc.DeleteFile("f-1"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if len(repo.deleted) != 1 {
		t.Fatalf("expected 1 deletion, got %d", len(repo.deleted))
	}
	if _, ok := storage.data["2026/01/01/id.png"]; ok {
		t.Fatal("expected storage file deleted")
	}
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }
