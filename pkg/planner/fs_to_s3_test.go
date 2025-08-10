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
			want:     "AAAAAAAAAAA=", // CRC64NVME of empty file
			wantErr:  false,
		},
		{
			name:     "hello world file",
			filename: "hello.txt",
			want:     "SoXXbx67KpE=", // CRC64NVME of "Hello, World!\n"
			wantErr:  false,
		},
		{
			name:     "multiline file",
			filename: "multiline.txt",
			want:     "TMMl8gG9v5U=", // CRC64NVME of "Line 1\nLine 2\nLine 3\n"
			wantErr:  false,
		},
		{
			name:     "known hash file",
			filename: "known_hash.txt",
			want:     "2yX60sjqiYo=", // CRC64NVME of "The quick brown fox jumps over the lazy dog\n"
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
			wantPrefix: "prefix/subdir", // Trailing slash should be removed by path.Clean
			wantErr:    false,
		},
		{
			name:       "bucket with multiple trailing slashes",
			uri:        "s3://mybucket/prefix///",
			wantBucket: "mybucket",
			wantPrefix: "prefix", // Multiple slashes should be normalized
			wantErr:    false,
		},
		{
			name:       "bucket with multiple internal slashes",
			uri:        "s3://mybucket/prefix//subdir///file",
			wantBucket: "mybucket",
			wantPrefix: "prefix/subdir/file", // Internal slashes should be normalized
			wantErr:    false,
		},
		{
			name:       "bucket with single slash",
			uri:        "s3://mybucket/",
			wantBucket: "mybucket",
			wantPrefix: "", // Single slash should result in empty prefix
			wantErr:    false,
		},
		{
			name:       "real-world example without trailing slash",
			uri:        "s3://example-bucket/media/images/files",
			wantBucket: "example-bucket",
			wantPrefix: "media/images/files",
			wantErr:    false,
		},
		{
			name:       "real-world example with trailing slash",
			uri:        "s3://example-bucket/media/images/files/",
			wantBucket: "example-bucket",
			wantPrefix: "media/images/files", // Should be normalized
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
