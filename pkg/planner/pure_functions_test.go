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
		bucket    string
		prefix    string
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
			bucket:    "test-bucket",
			prefix:    "prefix",
			want: []Item{
				{
					Action:    ActionUpload,
					LocalPath: "/local/file1.txt",
					Bucket:    "test-bucket",
					Key:       "prefix/file1.txt",
					Size:      100,
					Reason:    "new file",
				},
				{
					Action:    ActionUpload,
					LocalPath: "/local/file2.txt",
					Bucket:    "test-bucket",
					Key:       "prefix/file2.txt",
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
			bucket:    "test-bucket",
			prefix:    "prefix",
			want: []Item{
				{
					Action:    ActionUpload,
					LocalPath: "/local/file1.txt",
					Bucket:    "test-bucket",
					Key:       "prefix/file1.txt",
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
			bucket:    "test-bucket",
			prefix:    "prefix",
			want: []Item{
				{
					Action:    ActionUpload,
					LocalPath: "/local/file1.txt",
					Bucket:    "test-bucket",
					Key:       "prefix/file1.txt",
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
			bucket:    "test-bucket",
			prefix:    "prefix",
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
			bucket:    "test-bucket",
			prefix:    "prefix",
			want: []Item{
				{
					Action:    ActionDelete,
					LocalPath: "",
					Bucket:    "test-bucket",
					Key:       "prefix/file1.txt",
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
			bucket:    "test-bucket",
			prefix:    "",
			want: []Item{
				{
					Action:    ActionDelete,
					LocalPath: "",
					Bucket:    "test-bucket",
					Key:       "z.txt",
					Size:      300,
					Reason:    "deleted locally",
				},
				{
					Action:    ActionUpload,
					LocalPath: "/local/a.txt",
					Bucket:    "test-bucket",
					Key:       "a.txt",
					Size:      200,
					Reason:    "new file",
				},
				{
					Action:    ActionUpload,
					LocalPath: "/local/b.txt",
					Bucket:    "test-bucket",
					Key:       "b.txt",
					Size:      100,
					Reason:    "new file",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Phase3GeneratePlan(tt.phase1, tt.checksums, tt.localBase, tt.bucket, tt.prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Phase3GeneratePlan() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		want     bool
		wantErr  bool
	}{
		{
			name:     "no patterns",
			path:     "file.txt",
			patterns: []string{},
			want:     false,
			wantErr:  false,
		},
		{
			name:     "simple match",
			path:     "test.tmp",
			patterns: []string{"*.tmp"},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "simple no match",
			path:     "test.txt",
			patterns: []string{"*.tmp"},
			want:     false,
			wantErr:  false,
		},
		{
			name:     "multiple patterns with match",
			path:     "test.tmp",
			patterns: []string{"*.log", "*.tmp", "*.bak"},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "multiple patterns no match",
			path:     "test.txt",
			patterns: []string{"*.log", "*.tmp", "*.bak"},
			want:     false,
			wantErr:  false,
		},
		{
			name:     "directory match",
			path:     "node_modules/package.json",
			patterns: []string{"node_modules/*"},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "nested directory match",
			path:     "src/test/data.tmp",
			patterns: []string{"*.tmp"},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "exact path match",
			path:     "config/secret.key",
			patterns: []string{"config/secret.key"},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "directory prefix without wildcard",
			path:     "temp/file.txt",
			patterns: []string{"temp/"},
			want:     false,
			wantErr:  false,
		},
		{
			name:     "directory prefix with wildcard",
			path:     "temp/file.txt",
			patterns: []string{"temp/*"},
			want:     true,
			wantErr:  false,
		},
		// AWS S3 sync compatible behavior tests
		{
			name:     "aws compat: star matches across directories",
			path:     "_next/subdir/file.txt",
			patterns: []string{"_next/*"},
			want:     true, // With fnmatch, * matches path separators
			wantErr:  false,
		},
		{
			name:     "aws compat: deeply nested match",
			path:     "_next/subdir/deep/file.txt",
			patterns: []string{"_next/*"},
			want:     true, // With fnmatch, * matches path separators
			wantErr:  false,
		},
		{
			name:     "complex pattern",
			path:     "src/components/Button.test.tsx",
			patterns: []string{"*.test.*"},
			want:     true, // With fnmatch, * matches path separators
			wantErr:  false,
		},
		{
			name:     "case sensitive",
			path:     "File.TXT",
			patterns: []string{"*.txt"},
			want:     false,
			wantErr:  false,
		},
		{
			name:     "hidden files",
			path:     ".git/config",
			patterns: []string{".git/*"},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "hidden file pattern",
			path:     ".env",
			patterns: []string{".*"},
			want:     true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsExcluded(tt.path, tt.patterns)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsExcluded() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsExcluded() = %v, want %v", got, tt.want)
			}
		})
	}
}
