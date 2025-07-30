package s3client

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type AWSClient struct {
	client *s3.Client
}

func NewAWSClient(cfg aws.Config) *AWSClient {
	return &AWSClient{
		client: s3.NewFromConfig(cfg),
	}
}

func (c *AWSClient) ListObjects(ctx context.Context, bucket, prefix string) ([]ItemMetadata, error) {
	var items []ItemMetadata

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil || obj.Size == nil {
				continue
			}

			key := *obj.Key
			if prefix != "" {
				key = strings.TrimPrefix(key, prefix+"/")
			}

			items = append(items, ItemMetadata{
				Path:    key,
				Size:    aws.ToInt64(obj.Size),
				ModTime: aws.ToTime(obj.LastModified),
			})
		}
	}

	return items, nil
}

func (c *AWSClient) HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	resp, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to head object: %w", err)
	}

	info := &ObjectInfo{
		Size: aws.ToInt64(resp.ContentLength),
	}

	if resp.ChecksumSHA256 != nil {
		decoded, err := base64.StdEncoding.DecodeString(*resp.ChecksumSHA256)
		if err == nil {
			info.Checksum = fmt.Sprintf("%x", decoded)
		}
	}

	return info, nil
}

func (c *AWSClient) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, checksum string) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	}

	if checksum != "" {
		checksumBytes, err := hex.DecodeString(checksum)
		if err == nil {
			input.ChecksumSHA256 = aws.String(base64.StdEncoding.EncodeToString(checksumBytes))
			input.ChecksumAlgorithm = types.ChecksumAlgorithmSha256
		}
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

func (c *AWSClient) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}
