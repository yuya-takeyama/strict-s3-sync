package s3client

import (
	"fmt"
	"strings"
)

// ParseS3URI parses an S3 URI into bucket and prefix
func ParseS3URI(uri string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("invalid S3 URI: must start with s3://")
	}

	path := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) == 0 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid S3 URI: missing bucket name")
	}

	bucket = parts[0]
	if len(parts) > 1 {
		prefix = parts[1]
		// Ensure prefix ends with / if not empty
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
	}

	return bucket, prefix, nil
}
