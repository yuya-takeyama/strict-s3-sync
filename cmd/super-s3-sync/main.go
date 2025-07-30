package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
	"github.com/yuya-takeyama/super-s3-sync/internal/logging"
	"github.com/yuya-takeyama/super-s3-sync/internal/plan"
	"github.com/yuya-takeyama/super-s3-sync/internal/s3client"
	"github.com/yuya-takeyama/super-s3-sync/internal/walker"
	"github.com/yuya-takeyama/super-s3-sync/internal/worker"
)

type syncConfig struct {
	localPath   string
	s3URI       string
	excludes    []string
	delete      bool
	dryRun      bool
	concurrency int
	region      string
	quiet       bool
}

func main() {
	var cfg syncConfig

	rootCmd := &cobra.Command{
		Use:   "super-s3-sync <LocalPath> <S3Uri>",
		Short: "Sync files from local to S3 with SHA-256 based comparison",
		Long:  `A fast S3 sync tool that uses SHA-256 checksums for accurate synchronization.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.localPath = args[0]
			cfg.s3URI = args[1]

			if err := validateConfig(&cfg); err != nil {
				return err
			}

			ctx := context.Background()
			return run(ctx, &cfg)
		},
	}

	rootCmd.Flags().StringSliceVar(&cfg.excludes, "exclude", nil, "Exclude patterns (can be specified multiple times)")
	rootCmd.Flags().BoolVar(&cfg.delete, "delete", false, "Delete files in destination that don't exist in source")
	rootCmd.Flags().BoolVar(&cfg.dryRun, "dryrun", false, "Show what would be done without actually doing it")
	rootCmd.Flags().IntVar(&cfg.concurrency, "concurrency", 32, "Number of concurrent operations")
	rootCmd.Flags().StringVar(&cfg.region, "region", "", "AWS region (uses default if not specified)")
	rootCmd.Flags().BoolVar(&cfg.quiet, "quiet", false, "Suppress output")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func validateConfig(cfg *syncConfig) error {
	if cfg.localPath == "" {
		return fmt.Errorf("local path is required")
	}

	if !strings.HasPrefix(cfg.s3URI, "s3://") {
		return fmt.Errorf("S3 URI must start with s3://")
	}

	if cfg.concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}

	return nil
}

func run(ctx context.Context, cfg *syncConfig) error {
	startTime := time.Now()
	logger := logging.NewLogger(cfg.quiet)

	// Parse S3 URI
	bucket, prefix, err := s3client.ParseS3URI(cfg.s3URI)
	if err != nil {
		return err
	}

	logger.Info("Syncing %s to s3://%s/%s", cfg.localPath, bucket, prefix)

	// Create AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	if cfg.region != "" {
		awsCfg.Region = cfg.region
	}

	// Create S3 client
	client := s3client.NewClient(awsCfg)

	// Walk local files
	fileWalker, err := walker.NewWalker(cfg.localPath, cfg.excludes)
	if err != nil {
		return fmt.Errorf("create walker: %w", err)
	}

	localFiles, err := fileWalker.Walk()
	if err != nil {
		return fmt.Errorf("walk files: %w", err)
	}

	logger.Info("Found %d local files", len(localFiles))

	// Create sync plan
	planner := plan.NewPlanner(client, false) // skipMissingChecksum = false by default
	s3KeyFunc := func(relPath string) string {
		return walker.GetS3Key(prefix, relPath)
	}

	syncPlan, err := planner.Plan(ctx, localFiles, bucket, prefix, s3KeyFunc, cfg.delete, cfg.excludes)
	if err != nil {
		return fmt.Errorf("create sync plan: %w", err)
	}

	// Log plan summary
	var uploadCount, deleteCount int
	for _, item := range syncPlan {
		switch item.Action {
		case plan.ActionUpload:
			uploadCount++
		case plan.ActionDelete:
			deleteCount++
		}
	}

	logger.Info("Plan: %d uploads, %d deletes", uploadCount, deleteCount)

	if len(syncPlan) == 0 {
		logger.Info("Nothing to sync")
		return nil
	}

	// Execute plan
	pool := worker.NewPool(client, cfg.concurrency, cfg.quiet, cfg.dryRun)
	results, err := pool.Execute(ctx, syncPlan, bucket)
	if err != nil {
		return fmt.Errorf("execute sync: %w", err)
	}

	// Calculate stats
	var stats worker.Stats
	worker.UpdateStats(&stats, results)

	// Print errors
	var hasErrors bool
	for _, result := range results {
		if result.Error != nil {
			hasErrors = true
			logger.Error("%s: %v", result.Item.S3Key, result.Error)
		}
	}

	// Print summary
	duration := time.Since(startTime)
	logger.PrintSummary(stats.Uploaded, stats.Deleted, stats.Errors, stats.BytesUploaded, duration)

	if hasErrors {
		return fmt.Errorf("sync completed with %d errors", stats.Errors)
	}

	return nil
}