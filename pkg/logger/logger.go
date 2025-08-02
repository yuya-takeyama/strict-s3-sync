package logger

import (
	"fmt"
	"log"
)

// Logger is the unified logging interface for strict-s3-sync
type Logger interface {
	// User-facing operation logs
	Upload(localPath, s3Path string)
	Delete(s3Path string)
	Error(operation, path string, err error)

	// Internal debug logs (no-op by default)
	Debug(message string)
}

// SyncLogger handles logging for both normal execution and dry-run mode
type SyncLogger struct {
	IsDryRun bool
	IsQuiet  bool
}

func (l *SyncLogger) Upload(localPath, s3Path string) {
	if l.IsQuiet {
		return
	}

	if l.IsDryRun {
		fmt.Printf("(dryrun) upload: %s to %s\n", localPath, s3Path)
	} else {
		fmt.Printf("upload: %s to %s\n", localPath, s3Path)
	}
}

func (l *SyncLogger) Delete(s3Path string) {
	if l.IsQuiet {
		return
	}

	if l.IsDryRun {
		fmt.Printf("(dryrun) delete: %s\n", s3Path)
	} else {
		fmt.Printf("delete: %s\n", s3Path)
	}
}

func (l *SyncLogger) Error(operation, path string, err error) {
	// Always show errors, even in quiet mode
	fmt.Printf("error: %s %s: %v\n", operation, path, err)
}

func (l *SyncLogger) Debug(message string) {
	// No-op by default
}

// DebugLogger extends SyncLogger with debug output
type DebugLogger struct {
	SyncLogger
}

func (l *DebugLogger) Debug(message string) {
	log.Printf("[DEBUG] %s", message)
}
