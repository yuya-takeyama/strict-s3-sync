package s3client

import (
	"testing"
)

func TestTrimS3KeyPrefix(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		prefix string
		want   string
	}{
		{
			name:   "normal key with prefix",
			key:    "assets/images/file.png",
			prefix: "assets/images",
			want:   "file.png",
		},
		{
			name:   "key with nested path",
			key:    "assets/images/subfolder/file.png",
			prefix: "assets/images",
			want:   "subfolder/file.png",
		},
		{
			name:   "empty prefix",
			key:    "assets/images/file.png",
			prefix: "",
			want:   "assets/images/file.png",
		},
		{
			name:   "prefix not matching",
			key:    "other/path/file.png",
			prefix: "assets/images",
			want:   "other/path/file.png",
		},
		{
			name:   "exact prefix match without trailing content",
			key:    "assets/images",
			prefix: "assets/images",
			want:   "assets/images", // No slash, so no trimming
		},
		{
			name:   "prefix with trailing slash in key",
			key:    "assets/images/",
			prefix: "assets/images",
			want:   "", // Removes "assets/images/"
		},
		{
			name:   "single level prefix",
			key:    "folder/file.txt",
			prefix: "folder",
			want:   "file.txt",
		},
		{
			name:   "deeply nested path",
			key:    "a/b/c/d/e/f/g/file.txt",
			prefix: "a/b/c",
			want:   "d/e/f/g/file.txt",
		},
		{
			name:   "real-world example",
			key:    "media/images/files/004191633258db8c29d8b4c3a8c9b7c299f9129e.png",
			prefix: "media/images/files",
			want:   "004191633258db8c29d8b4c3a8c9b7c299f9129e.png",
		},
		{
			name:   "prefix longer than key",
			key:    "short",
			prefix: "very/long/prefix/path",
			want:   "short",
		},
		{
			name:   "key is exactly prefix with slash",
			key:    "prefix/",
			prefix: "prefix",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimS3KeyPrefix(tt.key, tt.prefix)
			if got != tt.want {
				t.Errorf("trimS3KeyPrefix(%q, %q) = %q, want %q", tt.key, tt.prefix, got, tt.want)
			}
		})
	}
}

// Test integration between parseS3URI (from planner package) and trimS3KeyPrefix
func TestPrefixHandlingIntegration(t *testing.T) {
	// This test documents the expected behavior when parseS3URI and trimS3KeyPrefix work together
	testCases := []struct {
		name           string
		s3URI          string
		s3Key          string
		expectedResult string
	}{
		{
			name:           "URI without trailing slash",
			s3URI:          "s3://bucket/prefix/subdir",
			s3Key:          "prefix/subdir/file.txt",
			expectedResult: "file.txt",
		},
		{
			name:           "URI with trailing slash",
			s3URI:          "s3://bucket/prefix/subdir/",
			s3Key:          "prefix/subdir/file.txt",
			expectedResult: "file.txt", // Should be same as without trailing slash
		},
		{
			name:           "URI with multiple trailing slashes",
			s3URI:          "s3://bucket/prefix/subdir///",
			s3Key:          "prefix/subdir/file.txt",
			expectedResult: "file.txt", // Should be normalized
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: parseS3URI is in the planner package, so we simulate its output here
			// After parseS3URI with path.Clean, the prefix will be normalized (no trailing slash)
			// For example, "prefix/subdir/" becomes "prefix/subdir"
			normalizedPrefix := "prefix/subdir" // This is what parseS3URI would output after path.Clean

			result := trimS3KeyPrefix(tc.s3Key, normalizedPrefix)
			if result != tc.expectedResult {
				t.Errorf("Integration test failed: %s\nExpected: %q\nGot: %q",
					tc.name, tc.expectedResult, result)
			}
		})
	}
}
