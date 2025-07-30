package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/yuya-takeyama/strict-s3-sync/internal/checksum"
	"github.com/yuya-takeyama/strict-s3-sync/internal/plan"
	"github.com/yuya-takeyama/strict-s3-sync/internal/s3client"
)

const (
	multipartThreshold = 64 * 1024 * 1024 // 64MB
	partSize           = 8 * 1024 * 1024  // 8MB
)

// Result represents the result of a sync operation
type Result struct {
	Item   plan.Item
	Error  error
	Output string
}

// Pool manages concurrent workers
type Pool struct {
	client      *s3client.Client
	concurrency int
	quiet       bool
	dryRun      bool
}

// NewPool creates a new worker pool
func NewPool(client *s3client.Client, concurrency int, quiet, dryRun bool) *Pool {
	return &Pool{
		client:      client,
		concurrency: concurrency,
		quiet:       quiet,
		dryRun:      dryRun,
	}
}

// Execute runs the sync plan
func (p *Pool) Execute(ctx context.Context, items []plan.Item, bucket string) ([]Result, error) {
	jobs := make(chan plan.Item, len(items))
	results := make(chan Result, len(items))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go p.worker(ctx, bucket, jobs, results, &wg)
	}

	// Send jobs
	for _, item := range items {
		jobs <- item
	}
	close(jobs)

	// Wait for workers to finish
	wg.Wait()
	close(results)

	// Collect results
	var allResults []Result
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults, nil
}

// worker processes jobs
func (p *Pool) worker(ctx context.Context, bucket string, jobs <-chan plan.Item, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for item := range jobs {
		select {
		case <-ctx.Done():
			results <- Result{Item: item, Error: ctx.Err()}
			return
		default:
		}

		var result Result
		result.Item = item

		switch item.Action {
		case plan.ActionUpload:
			output, err := p.upload(ctx, bucket, item)
			result.Output = output
			result.Error = err
		case plan.ActionDelete:
			output, err := p.delete(ctx, bucket, item)
			result.Output = output
			result.Error = err
		}

		results <- result
	}
}

// upload handles file upload
func (p *Pool) upload(ctx context.Context, bucket string, item plan.Item) (string, error) {
	output := fmt.Sprintf("upload: %s to s3://%s/%s", item.LocalPath, bucket, item.S3Key)

	if !p.quiet {
		fmt.Println(output)
	}

	if p.dryRun {
		return output, nil
	}

	// Open file
	file, err := os.Open(item.LocalPath)
	if err != nil {
		return output, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Use multipart for large files
	if item.Size > multipartThreshold {
		return output, p.multipartUpload(ctx, bucket, item, file)
	}

	// Single part upload
	checksumReader := checksum.NewTeeReaderWithChecksum(file)
	_, err = p.client.PutObject(ctx, bucket, item.S3Key, checksumReader, item.Size, types.ChecksumAlgorithmSha256)
	if err != nil {
		return output, fmt.Errorf("put object: %w", err)
	}

	return output, nil
}

// multipartUpload handles multipart upload
func (p *Pool) multipartUpload(ctx context.Context, bucket string, item plan.Item, file *os.File) error {
	// Create multipart upload
	createResp, err := p.client.CreateMultipartUpload(ctx, bucket, item.S3Key, types.ChecksumAlgorithmSha256)
	if err != nil {
		return fmt.Errorf("create multipart upload: %w", err)
	}

	uploadID := *createResp.UploadId
	var completedParts []types.CompletedPart
	var uploadErr error
	partNumber := int32(1)

	// Upload parts
	for {
		// Read part data
		partData := make([]byte, partSize)
		n, err := io.ReadFull(file, partData)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			uploadErr = fmt.Errorf("read part: %w", err)
			break
		}
		if n == 0 {
			break
		}

		// Upload part
		partResp, err := p.client.UploadPart(ctx, bucket, item.S3Key, uploadID, partNumber,
			&bytesReader{data: partData[:n]}, int64(n))
		if err != nil {
			uploadErr = fmt.Errorf("upload part %d: %w", partNumber, err)
			break
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       partResp.ETag,
			PartNumber: &partNumber,
		})

		partNumber++

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}

	// Complete or abort upload
	if uploadErr != nil {
		// TODO: Implement abort multipart upload
		return uploadErr
	}

	_, err = p.client.CompleteMultipartUpload(ctx, bucket, item.S3Key, uploadID, completedParts)
	if err != nil {
		return fmt.Errorf("complete multipart upload: %w", err)
	}

	return nil
}

// delete handles object deletion
func (p *Pool) delete(ctx context.Context, bucket string, item plan.Item) (string, error) {
	output := fmt.Sprintf("delete: s3://%s/%s", bucket, item.S3Key)

	if !p.quiet {
		fmt.Println(output)
	}

	if p.dryRun {
		return output, nil
	}

	_, err := p.client.DeleteObject(ctx, bucket, item.S3Key)
	if err != nil {
		return output, fmt.Errorf("delete object: %w", err)
	}

	return output, nil
}

// bytesReader implements io.Reader for byte slices
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Stats tracks sync statistics
type Stats struct {
	Uploaded      int64
	Deleted       int64
	Errors        int64
	BytesUploaded int64
}

// UpdateStats updates statistics from results
func UpdateStats(stats *Stats, results []Result) {
	for _, result := range results {
		if result.Error != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}

		switch result.Item.Action {
		case plan.ActionUpload:
			atomic.AddInt64(&stats.Uploaded, 1)
			atomic.AddInt64(&stats.BytesUploaded, result.Item.Size)
		case plan.ActionDelete:
			atomic.AddInt64(&stats.Deleted, 1)
		}
	}
}
