package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/executor"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/planner"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

var (
	dryRun         bool
	deleteFlag     bool
	excludes       []string
	includes       []string
	quiet          bool
	concurrency    int
	profile        string
	region         string
	resultJSONFile string
)

type SyncResult struct {
	Changes []FileChange  `json:"changes"`
	Summary Summary       `json:"summary"`
	Skipped []SkippedFile `json:"skipped"`
}

type SkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type FileChange struct {
	Action string `json:"action"`
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
	Error  string `json:"error,omitempty"`
}

type Summary struct {
	TotalFiles int `json:"total_files"`
	Created    int `json:"created"`
	Updated    int `json:"updated"`
	Deleted    int `json:"deleted"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "strict-s3-sync <LocalPath> <S3Uri>",
		Short: "Strict S3 synchronization tool using CRC64NVME checksums",
		Long: `strict-s3-sync is a reliable S3 sync tool that uses CRC64NVME checksums
for accurate file comparison, ensuring data integrity.`,
		Version: fmt.Sprintf("%s (commit: %s, built at: %s by %s)", version, commit, date, builtBy),
		Args:    cobra.ExactArgs(2),
		RunE:    run,
	}

	rootCmd.Flags().BoolVar(&dryRun, "dryrun", false, "Shows operations without executing")
	rootCmd.Flags().BoolVar(&deleteFlag, "delete", false, "Delete dest files not in source")
	rootCmd.Flags().StringSliceVar(&excludes, "exclude", nil, "Exclude patterns (multiple allowed)")
	rootCmd.Flags().StringSliceVar(&includes, "include", nil, "Include patterns (multiple allowed)")
	rootCmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress non-error output")
	rootCmd.Flags().IntVar(&concurrency, "concurrency", 32, "Number of concurrent operations")
	rootCmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use")
	rootCmd.Flags().StringVar(&region, "region", "", "AWS region (uses default if not specified)")
	rootCmd.Flags().StringVar(&resultJSONFile, "result-json-file", "", "Path to output result as JSON file")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	localPath := args[0]
	s3URI := args[1]

	if !strings.HasPrefix(s3URI, "s3://") {
		return fmt.Errorf("second argument must be an S3 URI (s3://bucket/prefix)")
	}

	ctx := context.Background()

	// Build config options
	var configOpts []func(*config.LoadOptions) error
	if profile != "" {
		configOpts = append(configOpts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		configOpts = append(configOpts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3client.NewAWSClient(cfg)

	// Create unified logger
	syncLogger := &logger.SyncLogger{
		IsDryRun: dryRun,
		IsQuiet:  quiet,
	}

	plnr := planner.NewFSToS3Planner(s3Client, syncLogger)

	source := planner.Source{
		Type: planner.SourceTypeFileSystem,
		Path: localPath,
	}

	dest := planner.Destination{
		Type: planner.DestTypeS3,
		Path: s3URI,
	}

	opts := planner.Options{
		DeleteEnabled: deleteFlag,
		Excludes:      excludes,
		Logger:        syncLogger,
	}

	items, err := plnr.Plan(ctx, source, dest, opts)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	// Get skipped files from planner
	skippedFiles := plnr.GetSkippedFiles()

	if len(items) == 0 {
		if resultJSONFile != "" {
			// Convert skipped files
			skipped := []SkippedFile{}
			for _, s := range skippedFiles {
				skipped = append(skipped, SkippedFile{
					Path:   s.Path,
					Reason: s.Reason,
				})
			}
			// Write empty result with skipped files
			result := SyncResult{
				Changes: []FileChange{},
				Summary: Summary{
					Skipped: len(skipped),
				},
				Skipped: skipped,
			}
			if err := writeJSONResult(resultJSONFile, result); err != nil {
				return fmt.Errorf("failed to write JSON result: %w", err)
			}
		}
		return nil
	}

	var syncResult SyncResult

	if dryRun {
		for _, item := range items {
			switch item.Action {
			case planner.ActionUpload:
				syncLogger.Upload(item.LocalPath, fmt.Sprintf("s3://%s/%s", item.Bucket, item.Key))
				action := getUploadActionName(item.Reason)
				change := FileChange{
					Action: action,
					Source: getAbsolutePath(item.LocalPath),
					Target: formatS3Path(item.Bucket, item.Key),
				}
				syncResult.Changes = append(syncResult.Changes, change)
				if action == "create" {
					syncResult.Summary.Created++
				} else {
					syncResult.Summary.Updated++
				}
			case planner.ActionDelete:
				syncLogger.Delete(fmt.Sprintf("s3://%s/%s", item.Bucket, item.Key))
				change := FileChange{
					Action: "delete",
					Target: formatS3Path(item.Bucket, item.Key),
				}
				syncResult.Changes = append(syncResult.Changes, change)
				syncResult.Summary.Deleted++
			}
		}
		syncResult.Summary.TotalFiles = len(items)

		// Convert and add skipped files
		for _, s := range skippedFiles {
			syncResult.Skipped = append(syncResult.Skipped, SkippedFile{
				Path:   s.Path,
				Reason: s.Reason,
			})
		}
		syncResult.Summary.Skipped = len(syncResult.Skipped)

		if resultJSONFile != "" {
			if err := writeJSONResult(resultJSONFile, syncResult); err != nil {
				return fmt.Errorf("failed to write JSON result: %w", err)
			}
		}
		return nil
	}

	exec := executor.NewExecutor(s3Client, syncLogger, concurrency)
	results := exec.Execute(ctx, items)

	var failed int
	for _, result := range results {
		var change FileChange
		if result.Error != nil {
			failed++
			log.Printf("Error: %s/%s: %v", result.Item.Bucket, result.Item.Key, result.Error)
			action := getActionName(result.Item.Action)
			if result.Item.Action == planner.ActionUpload {
				action = getUploadActionName(result.Item.Reason)
			}
			if result.Item.Action == planner.ActionUpload {
				change = FileChange{
					Action: action,
					Source: getAbsolutePath(result.Item.LocalPath),
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
					Error:  result.Error.Error(),
				}
			} else {
				change = FileChange{
					Action: action,
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
					Error:  result.Error.Error(),
				}
			}
			syncResult.Summary.Failed++
		} else {
			action := getActionName(result.Item.Action)
			if result.Item.Action == planner.ActionUpload {
				action = getUploadActionName(result.Item.Reason)
			}
			if result.Item.Action == planner.ActionUpload {
				change = FileChange{
					Action: action,
					Source: getAbsolutePath(result.Item.LocalPath),
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
				}
			} else {
				change = FileChange{
					Action: action,
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
				}
			}
			switch result.Item.Action {
			case planner.ActionUpload:
				if action == "create" {
					syncResult.Summary.Created++
				} else {
					syncResult.Summary.Updated++
				}
			case planner.ActionDelete:
				syncResult.Summary.Deleted++
			}
		}
		syncResult.Changes = append(syncResult.Changes, change)
	}
	syncResult.Summary.TotalFiles = len(results)

	// Convert and add skipped files
	for _, s := range skippedFiles {
		syncResult.Skipped = append(syncResult.Skipped, SkippedFile{
			Path:   s.Path,
			Reason: s.Reason,
		})
	}
	syncResult.Summary.Skipped = len(syncResult.Skipped)

	if resultJSONFile != "" {
		if err := writeJSONResult(resultJSONFile, syncResult); err != nil {
			return fmt.Errorf("failed to write JSON result: %w", err)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d operations failed", failed)
	}

	return nil
}

func writeJSONResult(path string, result SyncResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func getActionName(action planner.Action) string {
	switch action {
	case planner.ActionUpload:
		return "create" // Use getUploadActionName for accurate create/update distinction
	case planner.ActionDelete:
		return "delete"
	default:
		return "unknown"
	}
}

func getUploadActionName(reason string) string {
	if reason == "new file" {
		return "create"
	}
	return "update"
}

func getAbsolutePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path // fallback to original path
	}
	return absPath
}

func formatS3Path(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}
