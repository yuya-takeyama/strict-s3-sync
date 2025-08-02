package planner

import (
	"context"
	"time"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
)

type Planner interface {
	Plan(ctx context.Context, source Source, dest Destination, opts Options) ([]Item, error)
}

type SourceType string
type DestType string

const (
	SourceTypeFileSystem SourceType = "filesystem"
	SourceTypeS3         SourceType = "s3"

	DestTypeFileSystem DestType = "filesystem"
	DestTypeS3         DestType = "s3"
)

type ItemMetadata struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Checksum string
}

type Source struct {
	Type     SourceType
	Path     string
	Metadata []ItemMetadata
}

type Destination struct {
	Type     DestType
	Path     string
	Metadata []ItemMetadata
}

type Options struct {
	DeleteEnabled bool
	Excludes      []string
	Logger        logger.Logger
}

type Action string

const (
	ActionUpload Action = "upload"
	ActionDelete Action = "delete"
	ActionSkip   Action = "skip"
)

type Item struct {
	Action    Action
	LocalPath string
	S3Key     string
	Size      int64
	Reason    string
	Checksum  string
}

