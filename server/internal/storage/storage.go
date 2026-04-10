package storage

import "context"

// FileStorage is the interface for file upload backends.
// Both S3 and local filesystem implementations satisfy this interface.
type FileStorage interface {
	Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (url string, err error)
	Delete(ctx context.Context, key string)
	DeleteKeys(ctx context.Context, keys []string)
	KeyFromURL(rawURL string) string
}
