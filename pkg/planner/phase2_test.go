package planner

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

func TestPhase2CollectChecksums(t *testing.T) {
	tests := []struct {
		name         string
		items        []ItemRef
		localBase    string
		bucket       string
		prefix       string
		mockSetup    func(*mockS3Client)
		want         []ChecksumData
		wantErr      bool
		wantLogCalls int
	}{
		{
			name: "successful checksum collection",
			items: []ItemRef{
				{Path: "hello.txt", Size: 13},
			},
			localBase: "testdata",
			bucket:    "test-bucket",
			prefix:    "prefix",
			mockSetup: func(m *mockS3Client) {
				m.headObjectFunc = func(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error) {
					if req.Bucket == "test-bucket" && req.Key == "prefix/hello.txt" {
						return &s3client.ObjectInfo{
							Size:     13,
							Checksum: "abcdef123456",
						}, nil
					}
					return nil, fmt.Errorf("unexpected call")
				}
			},
			want: []ChecksumData{
				{
					ItemRef:        ItemRef{Path: "hello.txt", Size: 13},
					SourceChecksum: "4a85d76f1ebb2a91",
					DestChecksum:   "abcdef123456",
				},
			},
			wantErr:      false,
			wantLogCalls: 1,
		},
		{
			name: "multiple files",
			items: []ItemRef{
				{Path: "hello.txt", Size: 13},
				{Path: "empty.txt", Size: 0},
			},
			localBase: "testdata",
			bucket:    "test-bucket",
			prefix:    "",
			mockSetup: func(m *mockS3Client) {
				m.headObjectFunc = func(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error) {
					switch req.Key {
					case "hello.txt":
						return &s3client.ObjectInfo{
							Size:     13,
							Checksum: "remote-hello-checksum",
						}, nil
					case "empty.txt":
						return &s3client.ObjectInfo{
							Size:     0,
							Checksum: "remote-empty-checksum",
						}, nil
					}
					return nil, fmt.Errorf("unexpected key: %s", req.Key)
				}
			},
			want: []ChecksumData{
				{
					ItemRef:        ItemRef{Path: "hello.txt", Size: 13},
					SourceChecksum: "4a85d76f1ebb2a91",
					DestChecksum:   "remote-hello-checksum",
				},
				{
					ItemRef:        ItemRef{Path: "empty.txt", Size: 0},
					SourceChecksum: "0000000000000000",
					DestChecksum:   "remote-empty-checksum",
				},
			},
			wantErr:      false,
			wantLogCalls: 2,
		},
		{
			name: "local file not found",
			items: []ItemRef{
				{Path: "nonexistent.txt", Size: 100},
			},
			localBase: "testdata",
			bucket:    "test-bucket",
			prefix:    "",
			mockSetup: func(m *mockS3Client) {
				// No need to set up HeadObject as it won't be called
			},
			want:         nil,
			wantErr:      true,
			wantLogCalls: 0,
		},
		{
			name: "S3 HeadObject error",
			items: []ItemRef{
				{Path: "hello.txt", Size: 13},
			},
			localBase: "testdata",
			bucket:    "test-bucket",
			prefix:    "",
			mockSetup: func(m *mockS3Client) {
				m.headObjectFunc = func(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error) {
					return nil, fmt.Errorf("S3 error: access denied")
				}
			},
			want:         nil,
			wantErr:      true,
			wantLogCalls: 0,
		},
		{
			name:      "empty items list",
			items:     []ItemRef{},
			localBase: "testdata",
			bucket:    "test-bucket",
			prefix:    "",
			mockSetup: func(m *mockS3Client) {
				// No calls expected
			},
			want:         nil,
			wantErr:      false,
			wantLogCalls: 0,
		},
		{
			name: "with nested prefix",
			items: []ItemRef{
				{Path: "hello.txt", Size: 13},
			},
			localBase: "testdata",
			bucket:    "test-bucket",
			prefix:    "path/to/files",
			mockSetup: func(m *mockS3Client) {
				m.headObjectFunc = func(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error) {
					if req.Bucket == "test-bucket" && req.Key == path.Join("path/to/files", "hello.txt") {
						return &s3client.ObjectInfo{
							Size:     13,
							Checksum: "nested-checksum",
						}, nil
					}
					return nil, fmt.Errorf("unexpected call: bucket=%s, key=%s", req.Bucket, req.Key)
				}
			},
			want: []ChecksumData{
				{
					ItemRef:        ItemRef{Path: "hello.txt", Size: 13},
					SourceChecksum: "4a85d76f1ebb2a91",
					DestChecksum:   "nested-checksum",
				},
			},
			wantErr:      false,
			wantLogCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockS3Client{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			mockLog := &mockLogger{}
			planner := NewFSToS3Planner(mockClient, mockLog)

			got, err := planner.Phase2CollectChecksums(context.Background(), tt.items, tt.localBase, tt.bucket, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("Phase2CollectChecksums() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Phase2CollectChecksums() = %+v, want %+v", got, tt.want)
			}

			// Verify logging calls
			if len(mockLog.itemProcessedCalls) != tt.wantLogCalls {
				t.Errorf("Expected %d ItemProcessed calls, got %d", tt.wantLogCalls, len(mockLog.itemProcessedCalls))
			}

			// Verify phase start and complete were called correctly
			if !tt.wantErr && len(tt.items) > 0 {
				if len(mockLog.phaseStartCalls) != 1 {
					t.Errorf("Expected 1 PhaseStart call, got %d", len(mockLog.phaseStartCalls))
				} else if mockLog.phaseStartCalls[0].phase != "Phase2" {
					t.Errorf("Expected PhaseStart phase = Phase2, got %s", mockLog.phaseStartCalls[0].phase)
				}

				if len(mockLog.phaseCompleteCalls) != 1 {
					t.Errorf("Expected 1 PhaseComplete call, got %d", len(mockLog.phaseCompleteCalls))
				}
			}
		})
	}
}
