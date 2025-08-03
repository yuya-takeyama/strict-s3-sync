package planner

import "testing"

func TestIsExcludedPatterns(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		want     bool
	}{
		{
			name:     "hidden file at root",
			path:     ".hidden",
			patterns: []string{".*"},
			want:     true,
		},
		{
			name:     "hidden file in subdirectory",
			path:     "dir1/.gitignore",
			patterns: []string{".*"},
			want:     false, // ".*" only matches files starting with . at root
		},
		{
			name:     "hidden file in subdirectory with wildcard pattern",
			path:     "dir1/.gitignore",
			patterns: []string{"**/.*"},
			want:     true,
		},
		{
			name:     "specific directory pattern",
			path:     "dir1/file.txt",
			patterns: []string{"dir1/**"},
			want:     true,
		},
		{
			name:     "all txt files pattern",
			path:     "dir1/file.txt",
			patterns: []string{"**/*.txt"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsExcluded(tt.path, tt.patterns)
			if err != nil {
				t.Errorf("IsExcluded() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("IsExcluded(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.want)
			}
		})
	}
}
