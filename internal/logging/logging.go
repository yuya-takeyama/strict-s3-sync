package logging

import (
	"fmt"
	"os"
	"time"
)

// Logger provides structured logging
type Logger struct {
	quiet bool
}

// NewLogger creates a new logger
func NewLogger(quiet bool) *Logger {
	return &Logger{quiet: quiet}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	if !l.quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
}

// Debug logs a debug message (currently same as info)
func (l *Logger) Debug(format string, args ...interface{}) {
	if !l.quiet {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// PrintSummary prints a summary of the sync operation
func (l *Logger) PrintSummary(uploaded, deleted, errors int64, bytesUploaded int64, duration time.Duration) {
	if l.quiet && errors == 0 {
		return
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Uploaded: %d files (%s)\n", uploaded, formatBytes(bytesUploaded))
	fmt.Printf("Deleted: %d files\n", deleted)
	if errors > 0 {
		fmt.Printf("Errors: %d\n", errors)
	}
	fmt.Printf("Duration: %s\n", duration.Round(time.Millisecond))
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}