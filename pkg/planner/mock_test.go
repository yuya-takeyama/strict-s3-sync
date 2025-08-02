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

// mockLogger is a mock implementation of logger.Logger for testing
type mockLogger struct {
	uploadCalls []uploadCall
	deleteCalls []deleteCall
	errorCalls  []errorCall
	debugCalls  []string
}

type uploadCall struct {
	localPath string
	s3Path    string
}

type deleteCall struct {
	s3Path string
}

type errorCall struct {
	operation string
	path      string
	err       error
}

func (m *mockLogger) Upload(localPath, s3Path string) {
	m.uploadCalls = append(m.uploadCalls, uploadCall{localPath, s3Path})
}

func (m *mockLogger) Delete(s3Path string) {
	m.deleteCalls = append(m.deleteCalls, deleteCall{s3Path})
}

func (m *mockLogger) Error(operation, path string, err error) {
	m.errorCalls = append(m.errorCalls, errorCall{operation, path, err})
}

func (m *mockLogger) Debug(message string) {
	m.debugCalls = append(m.debugCalls, message)
}
