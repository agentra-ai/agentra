package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStorage is a FileStorage backend backed by MinIO or any S3-compatible object store.
type MinIOStorage struct {
	client    *minio.Client
	bucket    string
	endpoint  string
	useSSL    bool
	cdnDomain string // if set, returned URLs use this instead of the endpoint
}

// NewMinIOStorageFromEnv creates a MinIOStorage from environment variables.
// Returns nil if MINIO_ENDPOINT is not set.
//
// Environment variables:
//   - MINIO_ENDPOINT (required, e.g. "localhost:9000")
//   - MINIO_ACCESS_KEY / MINIO_SECRET_KEY (required)
//   - MINIO_BUCKET (required, e.g. "agentra-uploads")
//   - MINIO_USE_SSL (default: "false")
//   - MINIO_CDN_DOMAIN (optional, for CDN-fronted URLs)
func NewMinIOStorageFromEnv() *MinIOStorage {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		slog.Info("MINIO_ENDPOINT not set, file upload disabled")
		return nil
	}

	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		slog.Error("MINIO_ACCESS_KEY or MINIO_SECRET_KEY not set, file upload disabled")
		return nil
	}

	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		slog.Error("MINIO_BUCKET not set, file upload disabled")
		return nil
	}

	useSSL := false
	if ssl := os.Getenv("MINIO_USE_SSL"); ssl == "true" || ssl == "1" {
		useSSL = true
	}

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	}

	client, err := minio.New(endpoint, opts)
	if err != nil {
		slog.Error("failed to create MinIO client", "endpoint", endpoint, "error", err)
		return nil
	}

	cdnDomain := os.Getenv("MINIO_CDN_DOMAIN")

	slog.Info("MinIO storage initialized", "endpoint", endpoint, "bucket", bucket, "use_ssl", useSSL, "cdn_domain", cdnDomain)
	st := &MinIOStorage{
		client:    client,
		bucket:    bucket,
		endpoint:  endpoint,
		useSSL:    useSSL,
		cdnDomain: cdnDomain,
	}

	// Ensure the bucket exists (idempotent — safe to call on every startup).
	if err := st.EnsureBucket(context.Background()); err != nil {
		slog.Error("MinIO bucket creation failed, uploads may fail", "bucket", bucket, "error", err)
	}

	return st
}

// EnsureBucket creates the bucket if it does not exist (idempotent).
func (s *MinIOStorage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("MinIO BucketExists: %w", err)
	}
	if exists {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("MinIO MakeBucket: %w", err)
	}
	slog.Info("MinIO bucket created", "bucket", s.bucket)
	return nil
}

// KeyFromURL extracts the object key from a CDN or endpoint URL.
func (s *MinIOStorage) KeyFromURL(rawURL string) string {
	for _, prefix := range []string{
		"https://" + s.cdnDomain + "/",
		"https://" + s.endpoint + "/",
		"http://" + s.endpoint + "/",
		"https://" + s.bucket + "." + s.endpoint + "/",
		"http://" + s.bucket + "." + s.endpoint + "/",
	} {
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return rawURL
}

// Delete removes an object from the bucket.
func (s *MinIOStorage) Delete(ctx context.Context, key string) {
	if key == "" {
		return
	}
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		slog.Error("MinIO RemoveObject failed", "key", key, "error", err)
	}
}

// DeleteKeys removes multiple objects. Best-effort.
func (s *MinIOStorage) DeleteKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		s.Delete(ctx, key)
	}
}

// Upload uploads an object and returns the URL.
func (s *MinIOStorage) Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error) {
	safe := sanitizeFilename(filename)
	reader := bytes.NewReader(data)

	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType:        contentType,
		ContentDisposition: fmt.Sprintf(`inline; filename="%s"`, safe),
		UserMetadata:       map[string]string{"filename": safe},
	})
	if err != nil {
		return "", fmt.Errorf("MinIO PutObject: %w", err)
	}

	domain := s.endpoint
	protocol := "http"
	if s.useSSL {
		protocol = "https"
	}
	if s.cdnDomain != "" {
		domain = s.cdnDomain
	}
	link := fmt.Sprintf("%s://%s/%s", protocol, domain, key)
	return link, nil
}

// Download retrieves an object. Used internally; not part of FileStorage interface.
func (s *MinIOStorage) Download(ctx context.Context, key string) ([]byte, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("MinIO GetObject: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", fmt.Errorf("MinIO ReadAll: %w", err)
	}

	info, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("MinIO Stat: %w", err)
	}

	return data, info.ContentType, nil
}
