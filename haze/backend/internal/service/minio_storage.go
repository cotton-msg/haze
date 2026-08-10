package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	PublicURL string
}

type MinioStorage struct {
	client  *minio.Client
	bucket  string
	baseURL string
}

func NewMinioStorage(cfg MinioConfig) (*MinioStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
	}

	baseURL := cfg.PublicURL
	if baseURL == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s/%s", scheme, cfg.Endpoint, cfg.Bucket)
	}

	return &MinioStorage{client: client, bucket: cfg.Bucket, baseURL: strings.TrimSuffix(baseURL, "/")}, nil
}

func (s *MinioStorage) Save(path string, reader io.Reader) error {
	_, err := s.client.PutObject(context.Background(), s.bucket, path, reader, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

func (s *MinioStorage) Get(path string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(context.Background(), s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *MinioStorage) Delete(path string) error {
	return s.client.RemoveObject(context.Background(), s.bucket, path, minio.RemoveObjectOptions{})
}

func (s *MinioStorage) URL(path string) string {
	return fmt.Sprintf("%s/%s", s.baseURL, path)
}

// PresignedURL генерирует временную ссылку на объект с подписью (срок жизни).
func (s *MinioStorage) PresignedURL(path string, expires time.Duration) (string, error) {
	url, err := s.client.Presign(context.Background(), "GET", s.bucket, path, expires, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
