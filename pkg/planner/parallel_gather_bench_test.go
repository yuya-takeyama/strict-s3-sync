package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
)

// ベンチマーク用のディレクトリ構造を作成
func createBenchmarkDirTree(b *testing.B, tmpDir string, numDirs, filesPerDir int) {
	b.Helper()

	// ディレクトリとファイルを作成
	for i := 0; i < numDirs; i++ {
		dirPath := filepath.Join(tmpDir, fmt.Sprintf("dir%03d", i))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			b.Fatalf("Failed to create directory: %v", err)
		}

		// 各ディレクトリにファイルを作成
		for j := 0; j < filesPerDir; j++ {
			filePath := filepath.Join(dirPath, fmt.Sprintf("file%03d.txt", j))
			content := fmt.Sprintf("This is file %d in directory %d", j, i)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				b.Fatalf("Failed to create file: %v", err)
			}
		}

		// ネストしたディレクトリも作成
		subDirPath := filepath.Join(dirPath, "subdir")
		if err := os.MkdirAll(subDirPath, 0755); err != nil {
			b.Fatalf("Failed to create subdirectory: %v", err)
		}

		// サブディレクトリにもファイルを作成
		for j := 0; j < filesPerDir/2; j++ {
			filePath := filepath.Join(subDirPath, fmt.Sprintf("subfile%03d.txt", j))
			content := fmt.Sprintf("This is subfile %d in directory %d", j, i)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				b.Fatalf("Failed to create subfile: %v", err)
			}
		}
	}
}

// 順次版の実装（比較用）
func sequentialGatherLocalFiles(basePath string, excludes []string) ([]ItemMetadata, error) {
	var items []ItemMetadata

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		excluded, err := IsExcluded(relPath, excludes)
		if err != nil {
			return err
		}
		if excluded {
			return nil
		}

		items = append(items, ItemMetadata{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})

		return nil
	})

	return items, err
}

// ベンチマーク: 小規模ディレクトリ構造
func BenchmarkGatherLocalFiles_Small(b *testing.B) {
	tmpDir := b.TempDir()
	createBenchmarkDirTree(b, tmpDir, 10, 20) // 10 dirs × 20 files = 200 files + subdirs

	planner := &FSToS3Planner{
		client: nil,
		logger: &logger.SyncLogger{IsDryRun: false, IsQuiet: true},
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := sequentialGatherLocalFiles(tmpDir, []string{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := planner.parallelGatherLocalFiles(tmpDir, []string{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ベンチマーク: 中規模ディレクトリ構造
func BenchmarkGatherLocalFiles_Medium(b *testing.B) {
	tmpDir := b.TempDir()
	createBenchmarkDirTree(b, tmpDir, 50, 50) // 50 dirs × 50 files = 2500 files + subdirs

	planner := &FSToS3Planner{
		client: nil,
		logger: &logger.SyncLogger{IsDryRun: false, IsQuiet: true},
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := sequentialGatherLocalFiles(tmpDir, []string{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := planner.parallelGatherLocalFiles(tmpDir, []string{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ベンチマーク: 大規模ディレクトリ構造
func BenchmarkGatherLocalFiles_Large(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large benchmark in short mode")
	}

	tmpDir := b.TempDir()
	createBenchmarkDirTree(b, tmpDir, 100, 100) // 100 dirs × 100 files = 10000 files + subdirs

	planner := &FSToS3Planner{
		client: nil,
		logger: &logger.SyncLogger{IsDryRun: false, IsQuiet: true},
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := sequentialGatherLocalFiles(tmpDir, []string{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := planner.parallelGatherLocalFiles(tmpDir, []string{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ワーカー数による性能比較
func BenchmarkGatherLocalFiles_WorkerComparison(b *testing.B) {
	tmpDir := b.TempDir()
	createBenchmarkDirTree(b, tmpDir, 50, 50)

	workerCounts := []int{1, 2, 4, 8, 16, 32}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			// 環境変数を設定してワーカー数を制御
			os.Setenv("STRICT_S3_SYNC_WORKERS", fmt.Sprintf("%d", workers))
			defer os.Unsetenv("STRICT_S3_SYNC_WORKERS")

			planner := &FSToS3Planner{
				client: nil,
				logger: &logger.SyncLogger{IsDryRun: false, IsQuiet: true},
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := planner.parallelGatherLocalFiles(tmpDir, []string{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
