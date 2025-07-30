package executor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/planner"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

type ExecutionLogger interface {
	ItemStart(item planner.Item)
	ItemComplete(item planner.Item, err error)
}

type VerboseLogger struct{}

func (l *VerboseLogger) ItemStart(item planner.Item) {
	switch item.Action {
	case planner.ActionUpload:
		fmt.Printf("upload: %s to s3://%s\n", item.LocalPath, item.S3Key)
	case planner.ActionDelete:
		fmt.Printf("delete: s3://%s\n", item.S3Key)
	}
}

func (l *VerboseLogger) ItemComplete(item planner.Item, err error) {
	if err != nil {
		fmt.Printf("error: %s: %v\n", item.S3Key, err)
	}
}

type QuietLogger struct{}

func (l *QuietLogger) ItemStart(item planner.Item) {}

func (l *QuietLogger) ItemComplete(item planner.Item, err error) {
	if err != nil {
		fmt.Printf("error: %s: %v\n", item.S3Key, err)
	}
}

type Executor struct {
	client      s3client.Client
	logger      ExecutionLogger
	concurrency int
}

func NewExecutor(client s3client.Client, logger ExecutionLogger, concurrency int) *Executor {
	if logger == nil {
		logger = &QuietLogger{}
	}
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

			e.logger.ItemStart(itm)
			err := e.executeItem(ctx, itm)
			e.logger.ItemComplete(itm, err)

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

	err = e.client.PutObject(ctx, bucket, key, file, item.Size, item.Checksum)
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

	err = e.client.DeleteObject(ctx, bucket, key)
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
