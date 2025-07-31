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
	PutObject(ctx context.Context, req *PutObjectRequest) error
	DeleteObject(ctx context.Context, bucket, key string) error
}

type ObjectInfo struct {
	Size     int64
	Checksum string
}

type PutObjectRequest struct {
	Bucket      string
	Key         string
	Body        io.Reader
	Size        int64
	Checksum    string
	ContentType string
}
