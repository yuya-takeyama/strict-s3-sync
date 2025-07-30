package planner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/yuya-takeyama/super-s3-sync/pkg/s3client"
)

type FSToS3Planner struct {
	client s3client.Client
	logger PlanLogger
}

func NewFSToS3Planner(client s3client.Client, logger PlanLogger) *FSToS3Planner {
	if logger == nil {
		logger = &NullLogger{}
	}
	return &FSToS3Planner{
		client: client,
		logger: logger,
	}
}

func (p *FSToS3Planner) Plan(ctx context.Context, source Source, dest Destination, opts Options) ([]Item, error) {
	if source.Type != SourceTypeFileSystem {
		return nil, fmt.Errorf("source must be filesystem, got %s", source.Type)
	}
	if dest.Type != DestTypeS3 {
		return nil, fmt.Errorf("destination must be s3, got %s", dest.Type)
	}

	bucket, prefix, err := parseS3URI(dest.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 URI: %w", err)
	}

	localFiles, err := p.gatherLocalFiles(source.Path, opts.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to gather local files: %w", err)
	}

	s3ClientObjects, err := p.client.ListObjects(ctx, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 objects: %w", err)
	}

	s3Objects := make([]ItemMetadata, len(s3ClientObjects))
	for i, obj := range s3ClientObjects {
		s3Objects[i] = ItemMetadata{
			Path:     obj.Path,
			Size:     obj.Size,
			ModTime:  obj.ModTime,
			Checksum: obj.Checksum,
		}
	}

	phase1Result := Phase1Compare(localFiles, s3Objects, opts.DeleteEnabled)
	p.logger.PhaseComplete("Phase1", len(localFiles)+len(s3Objects))

	checksums, err := p.Phase2CollectChecksums(ctx, phase1Result.NeedChecksum, source.Path, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to collect checksums: %w", err)
	}

	items := Phase3GeneratePlan(phase1Result, checksums, source.Path, prefix)
	p.logger.PhaseComplete("Phase3", len(items))

	return items, nil
}

func (p *FSToS3Planner) gatherLocalFiles(basePath string, excludes []string) ([]ItemMetadata, error) {
	var items []ItemMetadata

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		excluded, err := isExcluded(relPath, excludes)
		if err != nil {
			return err
		}
		if excluded {
			return nil
		}

		items = append(items, ItemMetadata{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})

		return nil
	})

	return items, err
}

func (p *FSToS3Planner) Phase2CollectChecksums(ctx context.Context, items []ItemRef, localBase string, bucket string, prefix string) ([]ChecksumData, error) {
	p.logger.PhaseStart("Phase2", len(items))

	var checksums []ChecksumData
	for _, item := range items {
		localPath := filepath.Join(localBase, item.Path)
		sourceChecksum, err := calculateFileChecksum(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", localPath, err)
		}

		s3Key := filepath.Join(prefix, item.Path)
		objInfo, err := p.client.HeadObject(ctx, bucket, s3Key)
		if err != nil {
			return nil, fmt.Errorf("failed to head object %s: %w", s3Key, err)
		}

		checksums = append(checksums, ChecksumData{
			ItemRef:        item,
			SourceChecksum: sourceChecksum,
			DestChecksum:   objInfo.Checksum,
		})

		p.logger.ItemProcessed("Phase2", item.Path, "checksum")
	}

	p.logger.PhaseComplete("Phase2", len(checksums))
	return checksums, nil
}

type NullLogger struct{}

func (l *NullLogger) PhaseStart(phase string, totalItems int) {}

func (l *NullLogger) ItemProcessed(phase string, item string, action string) {}

func (l *NullLogger) PhaseComplete(phase string, processedItems int) {}

func parseS3URI(uri string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("URI must start with s3://")
	}

	path := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(path, "/", 2)

	bucket = parts[0]
	if len(parts) > 1 {
		prefix = parts[1]
	}

	if bucket == "" {
		return "", "", fmt.Errorf("bucket name cannot be empty")
	}

	return bucket, prefix, nil
}

func isExcluded(path string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func calculateFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
