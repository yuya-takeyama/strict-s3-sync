package plan

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/yuya-takeyama/strict-s3-sync/internal/checksum"
	"github.com/yuya-takeyama/strict-s3-sync/internal/s3client"
	"github.com/yuya-takeyama/strict-s3-sync/internal/walker"
)

// Action represents a sync action
type Action string

const (
	ActionUpload Action = "upload"
	ActionDelete Action = "delete"
	ActionSkip   Action = "skip"
)

// Item represents a sync plan item
type Item struct {
	Action         Action
	LocalPath      string // Full path for upload
	S3Key          string
	Size           int64
	Reason         string // Why this action was chosen
	ChecksumSHA256 string // For uploads, calculated on demand
}

// Planner creates sync plans
type Planner struct {
	client              *s3client.Client
	skipMissingChecksum bool
}

// NewPlanner creates a new planner
func NewPlanner(client *s3client.Client, skipMissingChecksum bool) *Planner {
	return &Planner{
		client:              client,
		skipMissingChecksum: skipMissingChecksum,
	}
}

// RemoteObject represents an S3 object
type RemoteObject struct {
	Key            string
	Size           int64
	ETag           string
	ChecksumSHA256 string // Will be populated by HeadObject if needed
}

// Plan creates a sync plan
func (p *Planner) Plan(ctx context.Context, localFiles []walker.FileInfo, bucket, prefix string, s3KeyFunc func(string) string, deleteEnabled bool, excludes []string) ([]Item, error) {
	// Create local file map
	localMap := make(map[string]walker.FileInfo)
	for _, f := range localFiles {
		s3Key := s3KeyFunc(f.RelPath)
		localMap[s3Key] = f
	}

	// Get remote objects
	remoteMap := make(map[string]RemoteObject)
	err := p.client.ListObjectsV2Pages(ctx, bucket, prefix, func(objects []types.Object) error {
		for _, obj := range objects {
			if obj.Key == nil || obj.Size == nil {
				continue
			}
			remoteMap[*obj.Key] = RemoteObject{
				Key:  *obj.Key,
				Size: *obj.Size,
				ETag: aws.ToString(obj.ETag),
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}

	var items []Item

	// Process local files
	for s3Key, localFile := range localMap {
		remote, exists := remoteMap[s3Key]

		if !exists {
			// New file
			items = append(items, Item{
				Action:    ActionUpload,
				LocalPath: localFile.Path,
				S3Key:     s3Key,
				Size:      localFile.Size,
				Reason:    "new file",
			})
		} else if localFile.Size != remote.Size {
			// Size differs
			items = append(items, Item{
				Action:    ActionUpload,
				LocalPath: localFile.Path,
				S3Key:     s3Key,
				Size:      localFile.Size,
				Reason:    fmt.Sprintf("size differs (local: %d, remote: %d)", localFile.Size, remote.Size),
			})
		} else {
			// Size matches, need to check checksum
			items = append(items, Item{
				Action:    ActionSkip, // Will be updated after checksum comparison
				LocalPath: localFile.Path,
				S3Key:     s3Key,
				Size:      localFile.Size,
				Reason:    "pending checksum comparison",
			})
		}
	}

	// Batch HEAD requests for size-matching files
	if err := p.compareChecksums(ctx, items, bucket, remoteMap); err != nil {
		return nil, err
	}

	// Process deletes
	if deleteEnabled {
		for s3Key := range remoteMap {
			if _, exists := localMap[s3Key]; !exists {
				// Check if this key should be excluded from deletion
				if !isExcludedFromDeletion(s3Key, prefix, excludes) {
					items = append(items, Item{
						Action: ActionDelete,
						S3Key:  s3Key,
						Reason: "not in source",
					})
				}
			}
		}
	}

	// Filter out skip actions
	var finalItems []Item
	for _, item := range items {
		if item.Action != ActionSkip {
			finalItems = append(finalItems, item)
		}
	}

	return finalItems, nil
}

// compareChecksums performs batch HEAD requests to compare checksums
func (p *Planner) compareChecksums(ctx context.Context, items []Item, bucket string, remoteMap map[string]RemoteObject) error {
	// Find items that need checksum comparison
	var needsComparison []int
	for i, item := range items {
		if item.Action == ActionSkip && item.Reason == "pending checksum comparison" {
			needsComparison = append(needsComparison, i)
		}
	}

	if len(needsComparison) == 0 {
		return nil
	}

	// Use goroutines for parallel HEAD requests
	const maxConcurrent = 50
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var headErrors []error

	for _, idx := range needsComparison {
		idx := idx // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			item := &items[idx]
			head, err := p.client.HeadObject(ctx, bucket, item.S3Key)
			if err != nil {
				mu.Lock()
				headErrors = append(headErrors, fmt.Errorf("head object %s: %w", item.S3Key, err))
				mu.Unlock()
				return
			}

			// Get S3 checksum
			var s3Checksum string
			if head.ChecksumSHA256 != nil {
				s3Checksum = *head.ChecksumSHA256
			}

			if s3Checksum == "" {
				// No checksum on S3 object
				if p.skipMissingChecksum {
					item.Reason = "skipped (no S3 checksum)"
					// Keep as skip
				} else {
					item.Action = ActionUpload
					item.Reason = "no S3 checksum (will add)"
				}
				return
			}

			// Calculate local checksum
			localChecksum, err := checksum.CalculateFileSHA256(item.LocalPath)
			if err != nil {
				mu.Lock()
				headErrors = append(headErrors, fmt.Errorf("calculate checksum %s: %w", item.LocalPath, err))
				mu.Unlock()
				return
			}

			// Compare
			if checksum.CompareChecksums(localChecksum, s3Checksum) {
				item.Reason = "checksum matches"
				// Keep as skip
			} else {
				item.Action = ActionUpload
				item.Reason = "checksum differs"
				item.ChecksumSHA256 = localChecksum // Store for later use
			}
		}()
	}

	wg.Wait()

	if len(headErrors) > 0 {
		return fmt.Errorf("checksum comparison failed: %v", headErrors[0])
	}

	return nil
}

// isExcludedFromDeletion checks if an S3 key should be excluded from deletion
func isExcludedFromDeletion(s3Key, prefix string, excludes []string) bool {
	// Remove prefix to get relative path
	relPath := s3Key
	if prefix != "" && len(s3Key) > len(prefix) {
		relPath = s3Key[len(prefix):]
	}

	// Check against exclude patterns
	for _, pattern := range excludes {
		if matched, _ := doublestar.Match(pattern, relPath); matched {
			return true
		}
	}
	return false
}
