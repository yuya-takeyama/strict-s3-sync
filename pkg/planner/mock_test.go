package planner

import (
	"context"
	"fmt"
	"io"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

// mockS3Client is a mock implementation of s3client.Client for testing
type mockS3Client struct {
	listObjectsFunc  func(ctx context.Context, bucket, prefix string) ([]s3client.ItemMetadata, error)
	headObjectFunc   func(ctx context.Context, bucket, key string) (*s3client.ObjectInfo, error)
	putObjectFunc    func(ctx context.Context, bucket, key string, body io.Reader, size int64, checksum string, contentType string) error
	deleteObjectFunc func(ctx context.Context, bucket, key string) error
}

func (m *mockS3Client) ListObjects(ctx context.Context, bucket, prefix string) ([]s3client.ItemMetadata, error) {
	if m.listObjectsFunc != nil {
		return m.listObjectsFunc(ctx, bucket, prefix)
	}
	return nil, fmt.Errorf("ListObjects not implemented")
}

func (m *mockS3Client) HeadObject(ctx context.Context, bucket, key string) (*s3client.ObjectInfo, error) {
	if m.headObjectFunc != nil {
		return m.headObjectFunc(ctx, bucket, key)
	}
	return nil, fmt.Errorf("HeadObject not implemented")
}

func (m *mockS3Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, checksum string, contentType string) error {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, bucket, key, body, size, checksum, contentType)
	}
	return fmt.Errorf("PutObject not implemented")
}

func (m *mockS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	if m.deleteObjectFunc != nil {
		return m.deleteObjectFunc(ctx, bucket, key)
	}
	return fmt.Errorf("DeleteObject not implemented")
}

// mockLogger is a mock implementation of PlanLogger for testing
type mockLogger struct {
	phaseStartCalls    []phaseStartCall
	itemProcessedCalls []itemProcessedCall
	phaseCompleteCalls []phaseCompleteCall
}

type phaseStartCall struct {
	phase      string
	totalItems int
}

type itemProcessedCall struct {
	phase  string
	item   string
	action string
}

type phaseCompleteCall struct {
	phase          string
	processedItems int
}

func (m *mockLogger) PhaseStart(phase string, totalItems int) {
	m.phaseStartCalls = append(m.phaseStartCalls, phaseStartCall{phase, totalItems})
}

func (m *mockLogger) ItemProcessed(phase string, item string, action string) {
	m.itemProcessedCalls = append(m.itemProcessedCalls, itemProcessedCall{phase, item, action})
}

func (m *mockLogger) PhaseComplete(phase string, processedItems int) {
	m.phaseCompleteCalls = append(m.phaseCompleteCalls, phaseCompleteCall{phase, processedItems})
}
