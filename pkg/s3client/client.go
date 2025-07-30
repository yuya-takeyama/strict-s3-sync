package s3client

import (
	"context"
	"io"
	"time"
)

type ItemMetadata struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Checksum string
}

type Client interface {
	ListObjects(ctx context.Context, bucket, prefix string) ([]ItemMetadata, error)
	HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)
	PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, checksum string) error
	DeleteObject(ctx context.Context, bucket, key string) error
}

type ObjectInfo struct {
	Size     int64
	Checksum string
}
