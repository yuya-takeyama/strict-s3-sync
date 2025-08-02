package planner

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/s3client"
)

// ベンチマーク用のモックS3クライアント
type benchMockS3Client struct {
	latency time.Duration
}

func (c *benchMockS3Client) ListObjects(ctx context.Context, req *s3client.ListObjectsRequest) ([]s3client.ItemMetadata, error) {
	return nil, nil
}

func (c *benchMockS3Client) HeadObject(ctx context.Context, req *s3client.HeadObjectRequest) (*s3client.ObjectInfo, error) {
	// S3 APIのレイテンシをシミュレート
	if c.latency > 0 {
		time.Sleep(c.latency)
	}
	return &s3client.ObjectInfo{
		Size:     1024,
		Checksum: "benchmark-checksum",
	}, nil
}

func (c *benchMockS3Client) PutObject(ctx context.Context, req *s3client.PutObjectRequest) error {
	return nil
}

func (c *benchMockS3Client) DeleteObject(ctx context.Context, req *s3client.DeleteObjectRequest) error {
	return nil
}

// ベンチマーク用のテストファイルを作成
func createBenchmarkFiles(t testing.TB, dir string, count int) []ItemRef {
	t.Helper()

	items := make([]ItemRef, count)
	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("bench_file_%05d.txt", i)
		path := filepath.Join(dir, filename)

		// 小さなファイルを作成（チェックサム計算のオーバーヘッドを測定）
		data := []byte(fmt.Sprintf("benchmark data for file %d", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		items[i] = ItemRef{
			Path: filename,
			Size: int64(len(data)),
		}
	}

	return items
}

func BenchmarkPhase2CollectChecksums(b *testing.B) {
	// ベンチマーク用の一時ディレクトリ
	tempDir, err := os.MkdirTemp("", "bench_phase2_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 異なるファイル数でベンチマーク
	fileCounts := []int{10, 100, 1000}

	for _, fileCount := range fileCounts {
		b.Run(fmt.Sprintf("files_%d", fileCount), func(b *testing.B) {
			// サブディレクトリを作成
			subDir := filepath.Join(tempDir, fmt.Sprintf("test_%d", fileCount))
			if err := os.MkdirAll(subDir, 0755); err != nil {
				b.Fatal(err)
			}

			// テストファイルを作成
			items := createBenchmarkFiles(b, subDir, fileCount)

			// モッククライアントとプランナーを作成
			mockClient := &benchMockS3Client{}
			planner := NewFSToS3Planner(mockClient, nil)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := planner.Phase2CollectChecksums(
					context.Background(),
					items,
					subDir,
					"bench-bucket",
					"bench-prefix",
				)
				if err != nil {
					b.Fatal(err)
				}
			}

			b.ReportMetric(float64(fileCount)/b.Elapsed().Seconds(), "files/sec")
		})
	}
}

// 並列度の違いによるパフォーマンス比較
// シリアル版の実装（比較用）
func (p *FSToS3Planner) Phase2CollectChecksumsSerial(ctx context.Context, items []ItemRef, localBase string, bucket string, prefix string) ([]ChecksumData, error) {
	if len(items) == 0 {
		return nil, nil
	}

	var checksums []ChecksumData
	for _, item := range items {
		localPath := filepath.Join(localBase, item.Path)
		sourceChecksum, err := calculateFileChecksum(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", localPath, err)
		}

		s3Key := path.Join(prefix, item.Path)
		objInfo, err := p.client.HeadObject(ctx, &s3client.HeadObjectRequest{
			Bucket: bucket,
			Key:    s3Key,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to head object %s: %w", s3Key, err)
		}

		checksums = append(checksums, ChecksumData{
			ItemRef:        item,
			SourceChecksum: sourceChecksum,
			DestChecksum:   objInfo.Checksum,
		})
	}

	return checksums, nil
}

// 並列版とシリアル版の比較ベンチマーク
func BenchmarkPhase2CollectChecksumsComparison(b *testing.B) {
	// 一時ディレクトリ
	tempDir, err := os.MkdirTemp("", "bench_comparison_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	fileCounts := []int{10, 100, 500}

	for _, fileCount := range fileCounts {
		// テストファイルを作成
		items := createBenchmarkFiles(b, tempDir, fileCount)
		mockClient := &benchMockS3Client{}
		planner := NewFSToS3Planner(mockClient, nil)

		b.Run(fmt.Sprintf("serial_%d_files", fileCount), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := planner.Phase2CollectChecksumsSerial(
					context.Background(),
					items,
					tempDir,
					"bench-bucket",
					"bench-prefix",
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(fileCount)/b.Elapsed().Seconds(), "files/sec")
		})

		b.Run(fmt.Sprintf("parallel_%d_files", fileCount), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := planner.Phase2CollectChecksums(
					context.Background(),
					items,
					tempDir,
					"bench-bucket",
					"bench-prefix",
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(fileCount)/b.Elapsed().Seconds(), "files/sec")
		})
	}
}

// 実際のS3 APIレイテンシをシミュレートした比較ベンチマーク
func BenchmarkPhase2CollectChecksumsWithLatency(b *testing.B) {
	// 一時ディレクトリ
	tempDir, err := os.MkdirTemp("", "bench_latency_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 典型的なS3 APIレイテンシ（リージョン内: 10ms, クロスリージョン: 50ms）
	latencies := []time.Duration{0, 10 * time.Millisecond, 50 * time.Millisecond}
	fileCount := 100 // 100ファイルで固定

	// テストファイルを作成
	items := createBenchmarkFiles(b, tempDir, fileCount)

	for _, latency := range latencies {
		latencyMs := latency.Milliseconds()

		b.Run(fmt.Sprintf("serial_latency_%dms", latencyMs), func(b *testing.B) {
			mockClient := &benchMockS3Client{latency: latency}
			planner := NewFSToS3Planner(mockClient, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := planner.Phase2CollectChecksumsSerial(
					context.Background(),
					items,
					tempDir,
					"bench-bucket",
					"bench-prefix",
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(fileCount)/b.Elapsed().Seconds(), "files/sec")
		})

		b.Run(fmt.Sprintf("parallel_latency_%dms", latencyMs), func(b *testing.B) {
			mockClient := &benchMockS3Client{latency: latency}
			planner := NewFSToS3Planner(mockClient, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := planner.Phase2CollectChecksums(
					context.Background(),
					items,
					tempDir,
					"bench-bucket",
					"bench-prefix",
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(fileCount)/b.Elapsed().Seconds(), "files/sec")
		})
	}
}

func BenchmarkPhase2CollectChecksumsWithDifferentConcurrency(b *testing.B) {
	// 一時ディレクトリ
	tempDir, err := os.MkdirTemp("", "bench_concurrency_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 1000ファイルで固定
	fileCount := 1000
	items := createBenchmarkFiles(b, tempDir, fileCount)

	// 異なる並列度でテスト（実際の実装では32固定だが、将来の改善のため）
	concurrencies := []int{1, 4, 8, 16, 32, 64}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("concurrency_%d", concurrency), func(b *testing.B) {
			// TODO: 並列度を変更できるようにPhase2CollectChecksumsを拡張した後に実装
			// 現在は32固定なので、このベンチマークは参考値

			mockClient := &benchMockS3Client{}
			planner := NewFSToS3Planner(mockClient, nil)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := planner.Phase2CollectChecksums(
					context.Background(),
					items,
					tempDir,
					"bench-bucket",
					"bench-prefix",
				)
				if err != nil {
					b.Fatal(err)
				}
			}

			b.ReportMetric(float64(fileCount)/b.Elapsed().Seconds(), "files/sec")
		})
	}
}
