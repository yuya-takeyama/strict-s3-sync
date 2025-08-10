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
	planJSONFile   string
	resultJSONFile string
)

// PlanResult represents the planned operations before execution
type PlanResult struct {
	Files   []PlanFile  `json:"files"`
	Summary PlanSummary `json:"summary"`
}

type PlanFile struct {
	Action string `json:"action"` // "skip", "create", "update", "delete"
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type PlanSummary struct {
	Skip   int `json:"skip"`
	Create int `json:"create"`
	Update int `json:"update"`
	Delete int `json:"delete"`
}

// SyncResult represents the actual execution results
type SyncResult struct {
	Files   []ResultFile  `json:"files"`
	Errors  []ErrorFile   `json:"errors"`
	Summary ResultSummary `json:"summary"`
}

type ResultFile struct {
	Action string `json:"action"` // "skipped", "created", "updated", "deleted"
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
}

type ErrorFile struct {
	Action string `json:"action"` // "create", "update", "delete"
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
	Error  string `json:"error"`
}

type ResultSummary struct {
	Skipped int `json:"skipped"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
	Failed  int `json:"failed"`
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
	rootCmd.Flags().StringVar(&planJSONFile, "plan-json-file", "", "Path to output plan as JSON file")
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

	// Output plan if requested
	if planJSONFile != "" {
		if err := writePlanResult(planJSONFile, items); err != nil {
			return fmt.Errorf("failed to write plan JSON: %w", err)
		}
	}

	if len(items) == 0 {
		if resultJSONFile != "" && !dryRun {
			// Write empty result for actual execution
			result := SyncResult{
				Files:   []ResultFile{},
				Errors:  []ErrorFile{},
				Summary: ResultSummary{},
			}
			if err := writeSyncResult(resultJSONFile, result); err != nil {
				return fmt.Errorf("failed to write result JSON: %w", err)
			}
		}
		return nil
	}

	if dryRun {
		// In dry-run mode, just log the operations
		for _, item := range items {
			switch item.Action {
			case planner.ActionUpload:
				syncLogger.Upload(item.LocalPath, fmt.Sprintf("s3://%s/%s", item.Bucket, item.Key))
			case planner.ActionDelete:
				syncLogger.Delete(fmt.Sprintf("s3://%s/%s", item.Bucket, item.Key))
			}
		}
		return nil
	}

	// Execute the plan
	exec := executor.NewExecutor(s3Client, syncLogger, concurrency)
	results := exec.Execute(ctx, items)

	// Process results
	syncResult := SyncResult{
		Files:  []ResultFile{},
		Errors: []ErrorFile{},
	}
	var failed int

	for _, result := range results {
		if result.Error != nil {
			failed++
			log.Printf("Error: %s/%s: %v", result.Item.Bucket, result.Item.Key, result.Error)

			// Add to errors array
			action := getActionName(result.Item.Action)
			if result.Item.Action == planner.ActionUpload {
				action = getUploadActionName(result.Item.Reason)
			}

			errorFile := ErrorFile{
				Action: action,
				Target: formatS3Path(result.Item.Bucket, result.Item.Key),
				Error:  result.Error.Error(),
			}
			if result.Item.Action == planner.ActionUpload {
				errorFile.Source = getAbsolutePath(result.Item.LocalPath)
			}
			syncResult.Errors = append(syncResult.Errors, errorFile)
			syncResult.Summary.Failed++
		} else {
			// Successful operations
			switch result.Item.Action {
			case planner.ActionUpload:
				action := getUploadActionName(result.Item.Reason)
				var actionPast string
				if action == "create" {
					actionPast = "created"
					syncResult.Summary.Created++
				} else {
					actionPast = "updated"
					syncResult.Summary.Updated++
				}
				file := ResultFile{
					Action: actionPast,
					Source: getAbsolutePath(result.Item.LocalPath),
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
				}
				syncResult.Files = append(syncResult.Files, file)
			case planner.ActionDelete:
				file := ResultFile{
					Action: "deleted",
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
				}
				syncResult.Files = append(syncResult.Files, file)
				syncResult.Summary.Deleted++
			case planner.ActionSkip:
				file := ResultFile{
					Action: "skipped",
					Source: getAbsolutePath(result.Item.LocalPath),
					Target: formatS3Path(result.Item.Bucket, result.Item.Key),
				}
				syncResult.Files = append(syncResult.Files, file)
				syncResult.Summary.Skipped++
			}
		}
	}

	if resultJSONFile != "" {
		if err := writeSyncResult(resultJSONFile, syncResult); err != nil {
			return fmt.Errorf("failed to write result JSON: %w", err)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d operations failed", failed)
	}

	return nil
}

func writePlanResult(path string, items []planner.Item) error {
	var plan PlanResult

	for _, item := range items {
		var file PlanFile
		switch item.Action {
		case planner.ActionUpload:
			action := getUploadActionName(item.Reason)
			file = PlanFile{
				Action: action,
				Source: getAbsolutePath(item.LocalPath),
				Target: formatS3Path(item.Bucket, item.Key),
				Reason: item.Reason,
			}
			if action == "create" {
				plan.Summary.Create++
			} else {
				plan.Summary.Update++
			}
		case planner.ActionDelete:
			file = PlanFile{
				Action: "delete",
				Target: formatS3Path(item.Bucket, item.Key),
				Reason: item.Reason,
			}
			plan.Summary.Delete++
		case planner.ActionSkip:
			file = PlanFile{
				Action: "skip",
				Source: getAbsolutePath(item.LocalPath),
				Target: formatS3Path(item.Bucket, item.Key),
				Reason: item.Reason,
			}
			plan.Summary.Skip++
		}
		plan.Files = append(plan.Files, file)
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func writeSyncResult(path string, result SyncResult) error {
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
	case planner.ActionSkip:
		return "skip"
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
