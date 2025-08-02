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
	ListObjects(ctx context.Context, req *ListObjectsRequest) ([]ItemMetadata, error)
	HeadObject(ctx context.Context, req *HeadObjectRequest) (*ObjectInfo, error)
	PutObject(ctx context.Context, req *PutObjectRequest) error
	DeleteObject(ctx context.Context, req *DeleteObjectRequest) error
}

type ObjectInfo struct {
	Size     int64
	Checksum string
}

type ListObjectsRequest struct {
	Bucket string
	Prefix string
}

type HeadObjectRequest struct {
	Bucket string
	Key    string
}

type PutObjectRequest struct {
	Bucket      string
	Key         string
	Body        io.Reader
	Size        int64
	Checksum    string
	ContentType string
}

type DeleteObjectRequest struct {
	Bucket string
	Key    string
}
