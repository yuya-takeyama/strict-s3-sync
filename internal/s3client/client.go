package s3client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	defaultMaxRetries = 5
	defaultBaseDelay  = 100 * time.Millisecond
	defaultMaxDelay   = 30 * time.Second
)

// Client wraps the S3 client with retry logic
type Client struct {
	s3Client   *s3.Client
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewClient creates a new S3 client wrapper
func NewClient(cfg aws.Config) *Client {
	return &Client{
		s3Client:   s3.NewFromConfig(cfg),
		maxRetries: defaultMaxRetries,
		baseDelay:  defaultBaseDelay,
		maxDelay:   defaultMaxDelay,
	}
}

// ListObjectsV2Pages lists objects with pagination support
func (c *Client) ListObjectsV2Pages(ctx context.Context, bucket, prefix string, fn func([]types.Object) error) error {
	paginator := s3.NewListObjectsV2Paginator(c.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := c.listObjectsV2WithRetry(ctx, paginator)
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}

		if err := fn(page.Contents); err != nil {
			return err
		}
	}

	return nil
}

// HeadObject retrieves object metadata
func (c *Client) HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	return c.headObjectWithRetry(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// PutObject uploads a single object
func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, checksumAlgorithm types.ChecksumAlgorithm) (*s3.PutObjectOutput, error) {
	return c.putObjectWithRetry(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		Body:              body,
		ContentLength:     aws.Int64(size),
		ChecksumAlgorithm: checksumAlgorithm,
	})
}

// CreateMultipartUpload initiates a multipart upload
func (c *Client) CreateMultipartUpload(ctx context.Context, bucket, key string, checksumAlgorithm types.ChecksumAlgorithm) (*s3.CreateMultipartUploadOutput, error) {
	return c.createMultipartUploadWithRetry(ctx, &s3.CreateMultipartUploadInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		ChecksumAlgorithm: checksumAlgorithm,
	})
}

// UploadPart uploads a part of a multipart upload
func (c *Client) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (*s3.UploadPartOutput, error) {
	return c.uploadPartWithRetry(ctx, &s3.UploadPartInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		UploadId:      aws.String(uploadID),
		PartNumber:    aws.Int32(partNumber),
		Body:          body,
		ContentLength: aws.Int64(size),
	})
}

// CompleteMultipartUpload completes a multipart upload
func (c *Client) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []types.CompletedPart) (*s3.CompleteMultipartUploadOutput, error) {
	return c.completeMultipartUploadWithRetry(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
}

// DeleteObject deletes an object
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error) {
	return c.deleteObjectWithRetry(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// Retry wrapper methods

func (c *Client) listObjectsV2WithRetry(ctx context.Context, paginator *s3.ListObjectsV2Paginator) (*s3.ListObjectsV2Output, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := paginator.NextPage(ctx)
		if err == nil {
			return output, nil
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) headObjectWithRetry(ctx context.Context, input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := c.s3Client.HeadObject(ctx, input)
		if err == nil {
			return output, nil
		}

		// HeadObject returns NotFound error which shouldn't be retried
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return nil, err
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) putObjectWithRetry(ctx context.Context, input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := c.s3Client.PutObject(ctx, input)
		if err == nil {
			return output, nil
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) createMultipartUploadWithRetry(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := c.s3Client.CreateMultipartUpload(ctx, input)
		if err == nil {
			return output, nil
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) uploadPartWithRetry(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := c.s3Client.UploadPart(ctx, input)
		if err == nil {
			return output, nil
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) completeMultipartUploadWithRetry(ctx context.Context, input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := c.s3Client.CompleteMultipartUpload(ctx, input)
		if err == nil {
			return output, nil
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) deleteObjectWithRetry(ctx context.Context, input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		output, err := c.s3Client.DeleteObject(ctx, input)
		if err == nil {
			return output, nil
		}

		if !c.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError checks if an error is retryable
func (c *Client) isRetryableError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "SlowDown", "ServiceUnavailable", "RequestTimeout", "RequestTimeoutException":
			return true
		}
		// Retry on 5xx errors
		if httpErr, ok := apiErr.(interface{ HTTPStatusCode() int }); ok {
			code := httpErr.HTTPStatusCode()
			return code >= 500 && code < 600
		}
	}
	// Also retry on network errors
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF)
}

// calculateDelay calculates the retry delay with exponential backoff and jitter
func (c *Client) calculateDelay(attempt int) time.Duration {
	base := float64(c.baseDelay)
	delay := base * math.Pow(2.0, float64(attempt))
	
	// Add jitter (±25%)
	jitter := delay * 0.25 * (2*rand.Float64() - 1)
	delay += jitter
	
	// Cap at maxDelay
	if delay > float64(c.maxDelay) {
		delay = float64(c.maxDelay)
	}
	
	return time.Duration(delay)
}