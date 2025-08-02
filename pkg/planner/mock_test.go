package planner

import (
	"context"
	"fmt"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

// mockS3Client is a mock implementation of s3client.Client for testing
type mockS3Client struct {
	listObjectsFunc  func(ctx context.Context, req *s3client.ListObjectsRequest) ([]s3client.ItemMetadata, error)
	headObjectFunc   func(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error)
	putObjectFunc    func(ctx context.Context, req *s3client.PutObjectRequest) error
	deleteObjectFunc func(ctx context.Context, req *s3client.DeleteObjectRequest) error
}

func (m *mockS3Client) ListObjects(ctx context.Context, req *s3client.ListObjectsRequest) ([]s3client.ItemMetadata, error) {
	if m.listObjectsFunc != nil {
		return m.listObjectsFunc(ctx, req)
	}
	return nil, fmt.Errorf("ListObjects not implemented")
}

func (m *mockS3Client) HeadObject(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error) {
	if m.headObjectFunc != nil {
		return m.headObjectFunc(ctx, req)
	}
	return nil, fmt.Errorf("HeadObject not implemented")
}

func (m *mockS3Client) PutObject(ctx context.Context, req *s3client.PutObjectRequest) error {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, req)
	}
	return fmt.Errorf("PutObject not implemented")
}

func (m *mockS3Client) DeleteObject(ctx context.Context, req *s3client.DeleteObjectRequest) error {
	if m.deleteObjectFunc != nil {
		return m.deleteObjectFunc(ctx, req)
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
