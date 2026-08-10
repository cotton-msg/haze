package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/google/uuid"
)

type StorageBackend interface {
	Save(path string, reader io.Reader) error
	Get(path string) (io.ReadCloser, error)
	Delete(path string) error
	URL(path string) string
	PresignedURL(path string, expires time.Duration) (string, error)
}

type LocalStorage struct {
	BasePath string
	BaseURL  string
}

func NewLocalStorage(basePath, baseURL string) *LocalStorage {
	os.MkdirAll(basePath, 0755)
	return &LocalStorage{BasePath: basePath, BaseURL: baseURL}
}

func (s *LocalStorage) Save(path string, reader io.Reader) error {
	fullPath := filepath.Join(s.BasePath, path)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, reader)
	return err
}

func (s *LocalStorage) Get(path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.BasePath, path))
}

func (s *LocalStorage) Delete(path string) error {
	return os.Remove(filepath.Join(s.BasePath, path))
}

func (s *LocalStorage) URL(path string) string {
	return fmt.Sprintf("%s/%s", s.BaseURL, path)
}

// PresignedURL для локального хранилища — прямой URL (файлы публичны).
func (s *LocalStorage) PresignedURL(path string, expires time.Duration) (string, error) {
	return s.URL(path), nil
}

var previewExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func isPreviewable(mime string) bool {
	_, ok := previewExts[mime]
	return ok
}

// FileRepository — интерфейс файлов для сервисного слоя.
type FileRepository interface {
	Create(file *models.File) error
	FindByID(id string) (*models.File, error)
	FindByMessageID(messageID string) ([]*models.File, error)
	Delete(id string) error
}

type MediaService struct {
	fileRepo FileRepository
	storage  StorageBackend
	maxSize  int64
	maxSizeP int64
}

func NewMediaService(fileRepo FileRepository, storage StorageBackend, maxSize, maxSizeP int64) *MediaService {
	return &MediaService{
		fileRepo: fileRepo,
		storage:  storage,
		maxSize:  maxSize,
		maxSizeP: maxSizeP,
	}
}

type UploadResult struct {
	File  *models.File `json:"file"`
	URL   string       `json:"url"`
	Thumb string       `json:"thumbnail,omitempty"`
}

func (s *MediaService) Upload(file multipart.File, header *multipart.FileHeader, isPremium bool) (*UploadResult, error) {
	limit := s.maxSize
	if isPremium {
		limit = s.maxSizeP
	}
	if header.Size > limit {
		return nil, fmt.Errorf("file too large: %d > %d", header.Size, limit)
	}

	ext := filepath.Ext(header.Filename)
	id := uuid.New().String()
	path := fmt.Sprintf("%s/%s%s", time.Now().Format("2006/01/02"), id, ext)

	if err := s.storage.Save(path, file); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	f := &models.File{
		URL:      s.storage.URL(path),
		MimeType: header.Header.Get("Content-Type"),
		Size:     header.Size,
		Name:     header.Filename,
	}

	if err := s.fileRepo.Create(f); err != nil {
		s.storage.Delete(path)
		return nil, fmt.Errorf("failed to save file record: %w", err)
	}

	result := &UploadResult{
		File: f,
		URL:  f.URL,
	}

	if isPreviewable(f.MimeType) {
		result.Thumb = f.URL
	}

	return result, nil
}

func (s *MediaService) GetFile(id string) (*models.File, error) {
	return s.fileRepo.FindByID(id)
}

// PresignURL возвращает временную ссылку на файл (для CDN/прямой отдачи).
func (s *MediaService) PresignURL(id string, expires time.Duration) (string, error) {
	f, err := s.fileRepo.FindByID(id)
	if err != nil {
		return "", err
	}
	path := strings.TrimPrefix(f.URL, s.storage.URL(""))
	return s.storage.PresignedURL(path, expires)
}

func (s *MediaService) GetFilePath(path string) string {
	if ls, ok := s.storage.(*LocalStorage); ok {
		return filepath.Join(ls.BasePath, path)
	}
	return path
}

func (s *MediaService) GetFileByPath(path string) (io.ReadCloser, error) {
	return s.storage.Get(strings.TrimPrefix(path, "/"))
}

func (s *MediaService) DeleteFile(id string) error {
	f, err := s.fileRepo.FindByID(id)
	if err != nil {
		return err
	}
	// Из URL восстанавливаем относительный путь объекта в хранилище.
	path := strings.TrimPrefix(f.URL, s.storage.URL(""))
	s.storage.Delete(path)
	return s.fileRepo.Delete(id)
}
