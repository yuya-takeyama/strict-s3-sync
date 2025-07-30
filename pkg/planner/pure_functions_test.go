package planner

import (
	"reflect"
	"testing"
)

func TestPhase1Compare(t *testing.T) {
	tests := []struct {
		name          string
		source        []ItemMetadata
		dest          []ItemMetadata
		deleteEnabled bool
		want          Phase1Result
	}{
		{
			name: "all new files",
			source: []ItemMetadata{
				{Path: "file1.txt", Size: 100},
				{Path: "file2.txt", Size: 200},
			},
			dest:          []ItemMetadata{},
			deleteEnabled: false,
			want: Phase1Result{
				NewItems: []ItemRef{
					{Path: "file1.txt", Size: 100},
					{Path: "file2.txt", Size: 200},
				},
				DeletedItems: []ItemRef{},
				SizeMismatch: []ItemRef{},
				NeedChecksum: []ItemRef{},
				Identical:    []ItemRef{},
			},
		},
		{
			name:   "all deleted files with delete enabled",
			source: []ItemMetadata{},
			dest: []ItemMetadata{
				{Path: "file1.txt", Size: 100},
				{Path: "file2.txt", Size: 200},
			},
			deleteEnabled: true,
			want: Phase1Result{
				NewItems: []ItemRef{},
				DeletedItems: []ItemRef{
					{Path: "file1.txt", Size: 100},
					{Path: "file2.txt", Size: 200},
				},
				SizeMismatch: []ItemRef{},
				NeedChecksum: []ItemRef{},
				Identical:    []ItemRef{},
			},
		},
		{
			name:   "deleted files ignored when delete disabled",
			source: []ItemMetadata{},
			dest: []ItemMetadata{
				{Path: "file1.txt", Size: 100},
			},
			deleteEnabled: false,
			want: Phase1Result{
				NewItems:     []ItemRef{},
				DeletedItems: []ItemRef{},
				SizeMismatch: []ItemRef{},
				NeedChecksum: []ItemRef{},
				Identical:    []ItemRef{},
			},
		},
		{
			name: "size mismatch",
			source: []ItemMetadata{
				{Path: "file1.txt", Size: 100},
			},
			dest: []ItemMetadata{
				{Path: "file1.txt", Size: 200},
			},
			deleteEnabled: false,
			want: Phase1Result{
				NewItems:     []ItemRef{},
				DeletedItems: []ItemRef{},
				SizeMismatch: []ItemRef{{Path: "file1.txt", Size: 100}},
				NeedChecksum: []ItemRef{},
				Identical:    []ItemRef{},
			},
		},
		{
			name: "need checksum verification",
			source: []ItemMetadata{
				{Path: "file1.txt", Size: 100},
			},
			dest: []ItemMetadata{
				{Path: "file1.txt", Size: 100},
			},
			deleteEnabled: false,
			want: Phase1Result{
				NewItems:     []ItemRef{},
				DeletedItems: []ItemRef{},
				SizeMismatch: []ItemRef{},
				NeedChecksum: []ItemRef{{Path: "file1.txt", Size: 100}},
				Identical:    []ItemRef{},
			},
		},
		{
			name: "identical files with matching checksums",
			source: []ItemMetadata{
				{Path: "file1.txt", Size: 100, Checksum: "abc123"},
			},
			dest: []ItemMetadata{
				{Path: "file1.txt", Size: 100, Checksum: "abc123"},
			},
			deleteEnabled: false,
			want: Phase1Result{
				NewItems:     []ItemRef{},
				DeletedItems: []ItemRef{},
				SizeMismatch: []ItemRef{},
				NeedChecksum: []ItemRef{},
				Identical:    []ItemRef{{Path: "file1.txt", Size: 100}},
			},
		},
		{
			name: "mixed scenario",
			source: []ItemMetadata{
				{Path: "new.txt", Size: 100},
				{Path: "same.txt", Size: 200, Checksum: "xyz"},
				{Path: "diff-size.txt", Size: 300},
				{Path: "need-check.txt", Size: 400},
			},
			dest: []ItemMetadata{
				{Path: "same.txt", Size: 200, Checksum: "xyz"},
				{Path: "diff-size.txt", Size: 350},
				{Path: "need-check.txt", Size: 400},
				{Path: "deleted.txt", Size: 500},
			},
			deleteEnabled: true,
			want: Phase1Result{
				NewItems: []ItemRef{
					{Path: "new.txt", Size: 100},
				},
				DeletedItems: []ItemRef{
					{Path: "deleted.txt", Size: 500},
				},
				SizeMismatch: []ItemRef{
					{Path: "diff-size.txt", Size: 300},
				},
				NeedChecksum: []ItemRef{
					{Path: "need-check.txt", Size: 400},
				},
				Identical: []ItemRef{
					{Path: "same.txt", Size: 200},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Phase1Compare(tt.source, tt.dest, tt.deleteEnabled)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Phase1Compare() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPhase3GeneratePlan(t *testing.T) {
	tests := []struct {
		name      string
		phase1    Phase1Result
		checksums []ChecksumData
		localBase string
		s3Prefix  string
		want      []Item
	}{
		{
			name: "new files only",
			phase1: Phase1Result{
				NewItems: []ItemRef{
					{Path: "file1.txt", Size: 100},
					{Path: "file2.txt", Size: 200},
				},
			},
			checksums: []ChecksumData{},
			localBase: "/local",
			s3Prefix:  "prefix",
			want: []Item{
				{
					Action:    ActionUpload,
					LocalPath: "/local/file1.txt",
					S3Key:     "prefix/file1.txt",
					Size:      100,
					Reason:    "new file",
				},
				{
					Action:    ActionUpload,
					LocalPath: "/local/file2.txt",
					S3Key:     "prefix/file2.txt",
					Size:      200,
					Reason:    "new file",
				},
			},
		},
		{
			name: "size mismatch files",
			phase1: Phase1Result{
				SizeMismatch: []ItemRef{
					{Path: "file1.txt", Size: 100},
				},
			},
			checksums: []ChecksumData{},
			localBase: "/local",
			s3Prefix:  "prefix",
			want: []Item{
				{
					Action:    ActionUpload,
					LocalPath: "/local/file1.txt",
					S3Key:     "prefix/file1.txt",
					Size:      100,
					Reason:    "size differs",
				},
			},
		},
		{
			name: "checksum differs",
			phase1: Phase1Result{
				NeedChecksum: []ItemRef{
					{Path: "file1.txt", Size: 100},
				},
			},
			checksums: []ChecksumData{
				{
					ItemRef:        ItemRef{Path: "file1.txt", Size: 100},
					SourceChecksum: "abc123",
					DestChecksum:   "def456",
				},
			},
			localBase: "/local",
			s3Prefix:  "prefix",
			want: []Item{
				{
					Action:    ActionUpload,
					LocalPath: "/local/file1.txt",
					S3Key:     "prefix/file1.txt",
					Size:      100,
					Reason:    "checksum differs",
				},
			},
		},
		{
			name: "checksum matches - no action",
			phase1: Phase1Result{
				NeedChecksum: []ItemRef{
					{Path: "file1.txt", Size: 100},
				},
			},
			checksums: []ChecksumData{
				{
					ItemRef:        ItemRef{Path: "file1.txt", Size: 100},
					SourceChecksum: "abc123",
					DestChecksum:   "abc123",
				},
			},
			localBase: "/local",
			s3Prefix:  "prefix",
			want:      []Item{},
		},
		{
			name: "deleted files",
			phase1: Phase1Result{
				DeletedItems: []ItemRef{
					{Path: "file1.txt", Size: 100},
				},
			},
			checksums: []ChecksumData{},
			localBase: "/local",
			s3Prefix:  "prefix",
			want: []Item{
				{
					Action:    ActionDelete,
					LocalPath: "",
					S3Key:     "prefix/file1.txt",
					Size:      100,
					Reason:    "deleted locally",
				},
			},
		},
		{
			name: "mixed actions with sorting",
			phase1: Phase1Result{
				NewItems: []ItemRef{
					{Path: "b.txt", Size: 100},
					{Path: "a.txt", Size: 200},
				},
				DeletedItems: []ItemRef{
					{Path: "z.txt", Size: 300},
				},
			},
			checksums: []ChecksumData{},
			localBase: "/local",
			s3Prefix:  "",
			want: []Item{
				{
					Action:    ActionDelete,
					LocalPath: "",
					S3Key:     "z.txt",
					Size:      300,
					Reason:    "deleted locally",
				},
				{
					Action:    ActionUpload,
					LocalPath: "/local/a.txt",
					S3Key:     "a.txt",
					Size:      200,
					Reason:    "new file",
				},
				{
					Action:    ActionUpload,
					LocalPath: "/local/b.txt",
					S3Key:     "b.txt",
					Size:      100,
					Reason:    "new file",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Phase3GeneratePlan(tt.phase1, tt.checksums, tt.localBase, tt.s3Prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Phase3GeneratePlan() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
