package s3client

import (
	"context"
	"fmt"
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

func (c *AWSClient) ListObjects(ctx context.Context, req *ListObjectsRequest) ([]ItemMetadata, error) {
	var items []ItemMetadata

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(req.Bucket),
		Prefix: aws.String(req.Prefix),
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
			if req.Prefix != "" {
				key = strings.TrimPrefix(key, req.Prefix+"/")
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

func (c *AWSClient) HeadObject(ctx context.Context, req *HeadObjectRequest) (*ObjectInfo, error) {
	resp, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       aws.String(req.Bucket),
		Key:          aws.String(req.Key),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to head object: %w", err)
	}

	info := &ObjectInfo{
		Size: aws.ToInt64(resp.ContentLength),
	}

	if resp.ChecksumCRC64NVME != nil {
		info.Checksum = *resp.ChecksumCRC64NVME
	}

	return info, nil
}

func (c *AWSClient) PutObject(ctx context.Context, req *PutObjectRequest) error {
	input := &s3.PutObjectInput{
		Bucket:            aws.String(req.Bucket),
		Key:               aws.String(req.Key),
		Body:              req.Body,
		ContentLength:     aws.Int64(req.Size),
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc64nvme,
	}

	if req.ContentType != "" {
		input.ContentType = aws.String(req.ContentType)
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

func (c *AWSClient) DeleteObject(ctx context.Context, req *DeleteObjectRequest) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}
