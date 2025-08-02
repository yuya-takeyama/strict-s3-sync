package executor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/planner"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)


type Executor struct {
	client      s3client.Client
	logger      logger.Logger
	concurrency int
}

func NewExecutor(client s3client.Client, logger logger.Logger, concurrency int) *Executor {
	if concurrency <= 0 {
		concurrency = 32
	}
	return &Executor{
		client:      client,
		logger:      logger,
		concurrency: concurrency,
	}
}

type Result struct {
	Item  planner.Item
	Error error
}

func (e *Executor) Execute(ctx context.Context, items []planner.Item) []Result {
	results := make([]Result, len(items))

	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, itm planner.Item) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Log the start of the operation
			switch itm.Action {
			case planner.ActionUpload:
				e.logger.Upload(itm.LocalPath, fmt.Sprintf("s3://%s", itm.S3Key))
			case planner.ActionDelete:
				e.logger.Delete(fmt.Sprintf("s3://%s", itm.S3Key))
			}

			err := e.executeItem(ctx, itm)
			
			// Log errors
			if err != nil {
				var operation string
				switch itm.Action {
				case planner.ActionUpload:
					operation = "upload"
				case planner.ActionDelete:
					operation = "delete"
				}
				e.logger.Error(operation, itm.S3Key, err)
			}

			results[idx] = Result{
				Item:  itm,
				Error: err,
			}
		}(i, item)
	}

	wg.Wait()
	return results
}

func (e *Executor) executeItem(ctx context.Context, item planner.Item) error {
	switch item.Action {
	case planner.ActionUpload:
		return e.uploadFile(ctx, item)
	case planner.ActionDelete:
		return e.deleteObject(ctx, item)
	default:
		return nil
	}
}

func (e *Executor) uploadFile(ctx context.Context, item planner.Item) error {
	file, err := os.Open(item.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	bucket, key, err := parseS3Key(item.S3Key)
	if err != nil {
		return err
	}

	contentType := guessContentType(item.LocalPath)
	err = e.client.PutObject(ctx, &s3client.PutObjectRequest{
		Bucket:      bucket,
		Key:         key,
		Body:        file,
		Size:        item.Size,
		Checksum:    item.Checksum,
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	return nil
}

func (e *Executor) deleteObject(ctx context.Context, item planner.Item) error {
	bucket, key, err := parseS3Key(item.S3Key)
	if err != nil {
		return err
	}

	err = e.client.DeleteObject(ctx, &s3client.DeleteObjectRequest{
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

func parseS3Key(s3Key string) (bucket, key string, err error) {
	parts := strings.SplitN(s3Key, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid S3 key format: %s", s3Key)
	}
	return parts[0], parts[1], nil
}
