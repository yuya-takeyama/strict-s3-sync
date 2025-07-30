package planner

import (
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
)

func Phase1Compare(source []ItemMetadata, dest []ItemMetadata, deleteEnabled bool) Phase1Result {
	sourceMap := make(map[string]ItemMetadata)
	for _, item := range source {
		sourceMap[item.Path] = item
	}

	destMap := make(map[string]ItemMetadata)
	for _, item := range dest {
		destMap[item.Path] = item
	}

	result := Phase1Result{
		NewItems:     []ItemRef{},
		DeletedItems: []ItemRef{},
		SizeMismatch: []ItemRef{},
		NeedChecksum: []ItemRef{},
		Identical:    []ItemRef{},
	}

	for path, srcItem := range sourceMap {
		if destItem, exists := destMap[path]; exists {
			ref := ItemRef{Path: path, Size: srcItem.Size}

			if srcItem.Size != destItem.Size {
				result.SizeMismatch = append(result.SizeMismatch, ref)
			} else if destItem.Checksum != "" && srcItem.Checksum != "" && srcItem.Checksum == destItem.Checksum {
				result.Identical = append(result.Identical, ref)
			} else {
				result.NeedChecksum = append(result.NeedChecksum, ref)
			}
		} else {
			result.NewItems = append(result.NewItems, ItemRef{
				Path: path,
				Size: srcItem.Size,
			})
		}
	}

	if deleteEnabled {
		for path, destItem := range destMap {
			if _, exists := sourceMap[path]; !exists {
				result.DeletedItems = append(result.DeletedItems, ItemRef{
					Path: path,
					Size: destItem.Size,
				})
			}
		}
	}

	sortPhase1Result(&result)
	return result
}

func Phase3GeneratePlan(phase1 Phase1Result, checksums []ChecksumData, localBase string, s3Prefix string) []Item {
	items := []Item{}

	for _, ref := range phase1.NewItems {
		items = append(items, Item{
			Action:    ActionUpload,
			LocalPath: filepath.Join(localBase, ref.Path),
			S3Key:     filepath.Join(s3Prefix, ref.Path),
			Size:      ref.Size,
			Reason:    "new file",
		})
	}

	for _, ref := range phase1.SizeMismatch {
		items = append(items, Item{
			Action:    ActionUpload,
			LocalPath: filepath.Join(localBase, ref.Path),
			S3Key:     filepath.Join(s3Prefix, ref.Path),
			Size:      ref.Size,
			Reason:    "size differs",
		})
	}

	checksumMap := make(map[string]ChecksumData)
	for _, cs := range checksums {
		checksumMap[cs.ItemRef.Path] = cs
	}

	for _, ref := range phase1.NeedChecksum {
		if cs, exists := checksumMap[ref.Path]; exists {
			if cs.SourceChecksum != cs.DestChecksum {
				items = append(items, Item{
					Action:    ActionUpload,
					LocalPath: filepath.Join(localBase, ref.Path),
					S3Key:     filepath.Join(s3Prefix, ref.Path),
					Size:      ref.Size,
					Reason:    "checksum differs",
				})
			}
		}
	}

	for _, ref := range phase1.DeletedItems {
		items = append(items, Item{
			Action:    ActionDelete,
			LocalPath: "",
			S3Key:     filepath.Join(s3Prefix, ref.Path),
			Size:      ref.Size,
			Reason:    "deleted locally",
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Action != items[j].Action {
			return items[i].Action < items[j].Action
		}
		return items[i].S3Key < items[j].S3Key
	})

	return items
}

func sortPhase1Result(result *Phase1Result) {
	sortItemRefs := func(refs []ItemRef) {
		sort.Slice(refs, func(i, j int) bool {
			return refs[i].Path < refs[j].Path
		})
	}

	sortItemRefs(result.NewItems)
	sortItemRefs(result.DeletedItems)
	sortItemRefs(result.SizeMismatch)
	sortItemRefs(result.NeedChecksum)
	sortItemRefs(result.Identical)
}

func IsExcluded(path string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
