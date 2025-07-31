package planner

import (
	"path/filepath"
	"testing"
)

func TestCalculateFileChecksum(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{
			name:     "empty file",
			filename: "empty.txt",
			want:     "0000000000000000", // CRC64NVME of empty file
			wantErr:  false,
		},
		{
			name:     "hello world file",
			filename: "hello.txt",
			want:     "d0026cc17976e73b", // CRC64NVME of "Hello, World!"
			wantErr:  false,
		},
		{
			name:     "multiline file",
			filename: "multiline.txt",
			want:     "620025a26b6a9bc4", // CRC64NVME of "Line 1\nLine 2\nLine 3"
			wantErr:  false,
		},
		{
			name:     "known hash file",
			filename: "known_hash.txt",
			want:     "f1b13f4ba1f1d0b5", // CRC64NVME of "The quick brown fox jumps over the lazy dog"
			wantErr:  false,
		},
		{
			name:     "non-existent file",
			filename: "does_not_exist.txt",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("testdata", tt.filename)
			got, err := calculateFileChecksum(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateFileChecksum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("calculateFileChecksum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantBucket string
		wantPrefix string
		wantErr    bool
	}{
		{
			name:       "bucket only",
			uri:        "s3://mybucket",
			wantBucket: "mybucket",
			wantPrefix: "",
			wantErr:    false,
		},
		{
			name:       "bucket with prefix",
			uri:        "s3://mybucket/prefix",
			wantBucket: "mybucket",
			wantPrefix: "prefix",
			wantErr:    false,
		},
		{
			name:       "bucket with nested prefix",
			uri:        "s3://mybucket/prefix/subdir/",
			wantBucket: "mybucket",
			wantPrefix: "prefix/subdir/",
			wantErr:    false,
		},
		{
			name:       "invalid scheme",
			uri:        "http://mybucket/prefix",
			wantBucket: "",
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "no scheme",
			uri:        "mybucket/prefix",
			wantBucket: "",
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "empty bucket",
			uri:        "s3:///prefix",
			wantBucket: "",
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "just scheme",
			uri:        "s3://",
			wantBucket: "",
			wantPrefix: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBucket, gotPrefix, err := parseS3URI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseS3URI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBucket != tt.wantBucket {
				t.Errorf("parseS3URI() gotBucket = %v, want %v", gotBucket, tt.wantBucket)
			}
			if gotPrefix != tt.wantPrefix {
				t.Errorf("parseS3URI() gotPrefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
		})
	}
}
