package executor

import (
	"mime"
	"path/filepath"
)

func guessContentType(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ""
	}

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return ""
	}

	return contentType
}
