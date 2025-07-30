package walker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// FileInfo represents a local file
type FileInfo struct {
	Path    string // Absolute path
	RelPath string // Relative path from root
	Size    int64
	ModTime int64 // Unix timestamp
	Mode    os.FileMode
}

// Walker walks local files with exclude pattern support
type Walker struct {
	root     string
	excludes []string
}

// NewWalker creates a new file walker
func NewWalker(root string, excludes []string) (*Walker, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	// Validate root exists and is a directory
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", absRoot)
	}

	return &Walker{
		root:     absRoot,
		excludes: excludes,
	}, nil
}

// Walk walks the file tree and returns matching files
func (w *Walker) Walk() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(w.root, path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		// Convert to forward slashes for pattern matching
		relPathForward := filepath.ToSlash(relPath)

		// Check excludes
		if w.isExcluded(relPathForward) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("get file info: %w", err)
		}

		files = append(files, FileInfo{
			Path:    path,
			RelPath: relPath,
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
			Mode:    info.Mode(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return files, nil
}

// isExcluded checks if a path matches any exclude pattern
func (w *Walker) isExcluded(path string) bool {
	for _, pattern := range w.excludes {
		// Handle directory patterns (ending with /)
		if strings.HasSuffix(pattern, "/") {
			// Check if path is under this directory
			dirPattern := strings.TrimSuffix(pattern, "/")
			if matched, _ := doublestar.Match(dirPattern, path); matched {
				return true
			}
			// Also check if any parent directory matches
			parts := strings.Split(path, "/")
			for i := 1; i <= len(parts); i++ {
				subPath := strings.Join(parts[:i], "/")
				if matched, _ := doublestar.Match(dirPattern, subPath); matched {
					return true
				}
			}
		} else {
			// Regular file pattern
			if matched, _ := doublestar.Match(pattern, path); matched {
				return true
			}
		}
	}
	return false
}

// GetS3Key converts a local relative path to an S3 key
func GetS3Key(prefix, relPath string) string {
	// Convert to forward slashes
	s3Path := filepath.ToSlash(relPath)
	
	if prefix == "" {
		return s3Path
	}
	
	// Ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	
	return prefix + s3Path
}