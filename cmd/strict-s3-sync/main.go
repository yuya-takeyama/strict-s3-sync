package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/executor"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/planner"
	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

var (
	dryRun      bool
	deleteFlag  bool
	excludes    []string
	includes    []string
	quiet       bool
	concurrency int
	profile     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "strict-s3-sync <LocalPath> <S3Uri>",
		Short: "Strict S3 synchronization tool using SHA-256 checksums",
		Long: `strict-s3-sync is a reliable S3 sync tool that uses SHA-256 checksums
for accurate file comparison, ensuring data integrity.`,
		Args: cobra.ExactArgs(2),
		RunE: run,
	}

	rootCmd.Flags().BoolVar(&dryRun, "dryrun", false, "Shows operations without executing")
	rootCmd.Flags().BoolVar(&deleteFlag, "delete", false, "Delete dest files not in source")
	rootCmd.Flags().StringSliceVar(&excludes, "exclude", nil, "Exclude patterns (multiple allowed)")
	rootCmd.Flags().StringSliceVar(&includes, "include", nil, "Include patterns (multiple allowed)")
	rootCmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress non-error output")
	rootCmd.Flags().IntVar(&concurrency, "concurrency", 32, "Number of concurrent operations")
	rootCmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use")

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

	var cfg aws.Config
	var err error
	if profile != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	} else {
		cfg, err = config.LoadDefaultConfig(ctx)
	}
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

	if len(items) == 0 {
		if !quiet {
			log.Println("No changes needed")
		}
		return nil
	}

	if dryRun {
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

	exec := executor.NewExecutor(s3Client, syncLogger, concurrency)
	results := exec.Execute(ctx, items)

	var failed int
	for _, result := range results {
		if result.Error != nil {
			failed++
			log.Printf("Error: %s/%s: %v", result.Item.Bucket, result.Item.Key, result.Error)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d operations failed", failed)
	}

	return nil
}
