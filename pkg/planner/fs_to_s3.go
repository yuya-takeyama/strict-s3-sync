package planner

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

// CRC64NVME polynomial as per AWS S3 specification
var crc64NVMETable = crc64.MakeTable(0x9a6c9329ac4bc9b5)

type FSToS3Planner struct {
	client s3client.Client
	logger logger.Logger
}

func NewFSToS3Planner(client s3client.Client, logger logger.Logger) *FSToS3Planner {
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

	s3ClientObjects, err := p.client.ListObjects(ctx, &s3client.ListObjectsRequest{
		Bucket: bucket,
		Prefix: prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 objects: %w", err)
	}

	s3Objects := []ItemMetadata{}
	for _, obj := range s3ClientObjects {
		// Apply exclude patterns to S3 objects
		excluded, err := IsExcluded(obj.Path, opts.Excludes)
		if err != nil {
			return nil, fmt.Errorf("failed to check exclude pattern for %s: %w", obj.Path, err)
		}
		if excluded {
			continue
		}

		s3Objects = append(s3Objects, ItemMetadata{
			Path:     obj.Path,
			Size:     obj.Size,
			ModTime:  obj.ModTime,
			Checksum: obj.Checksum,
		})
	}

	phase1Result := Phase1Compare(localFiles, s3Objects, opts.DeleteEnabled)

	checksums, err := p.Phase2CollectChecksums(ctx, phase1Result.NeedChecksum, source.Path, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to collect checksums: %w", err)
	}

	s3Prefix := bucket
	if prefix != "" {
		s3Prefix = bucket + "/" + prefix
	}
	items := Phase3GeneratePlan(phase1Result, checksums, source.Path, s3Prefix)

	// Calculate checksums for upload items
	for i, item := range items {
		if item.Action == ActionUpload {
			checksum, err := calculateFileChecksum(item.LocalPath)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate checksum for %s: %w", item.LocalPath, err)
			}
			items[i].Checksum = checksum
		}
	}

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

		excluded, err := IsExcluded(relPath, excludes)
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

	var checksums []ChecksumData
	for _, item := range items {
		localPath := filepath.Join(localBase, item.Path)
		sourceChecksum, err := calculateFileChecksum(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", localPath, err)
		}

		s3Key := path.Join(prefix, item.Path)
		objInfo, err := p.client.HeadObject(ctx, &s3client.HeadObjectRequest{
			Bucket: bucket,
			Key:    s3Key,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to head object %s: %w", s3Key, err)
		}

		checksums = append(checksums, ChecksumData{
			ItemRef:        item,
			SourceChecksum: sourceChecksum,
			DestChecksum:   objInfo.Checksum,
		})
	}

	return checksums, nil
}

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

func calculateFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := crc64.New(crc64NVMETable)
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}
