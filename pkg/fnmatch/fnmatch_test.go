package fnmatch

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		// Basic wildcards
		{"star matches everything", "*", "anything", true},
		{"star matches empty", "*", "", true},
		{"star matches path separator", "*", "path/to/file", true},
		{"multiple stars", "**", "path/to/file", true},

		// Question mark
		{"question matches single char", "?", "a", true},
		{"question doesn't match empty", "?", "", false},
		{"question matches any char", "???", "abc", true},

		// Path separator handling (Python fnmatch behavior)
		{"star matches across directories", "_next/*", "_next/file.txt", true},
		{"star matches nested directories", "_next/*", "_next/subdir/file.txt", true},
		{"star matches deeply nested", "_next/*", "_next/subdir/deep/file.txt", true},

		// Character classes
		{"char class single", "[abc]", "a", true},
		{"char class single", "[abc]", "b", true},
		{"char class single", "[abc]", "d", false},
		{"char class range", "[a-z]", "m", true},
		{"char class range", "[a-z]", "A", false},
		{"negated char class", "[!abc]", "d", true},
		{"negated char class", "[!abc]", "a", false},

		// Complex patterns
		{"prefix and star", "test*", "test", true},
		{"prefix and star", "test*", "testing", true},
		{"prefix and star", "test*", "test/file", true},
		{"star in middle", "test*file", "test123file", true},
		{"star in middle with path", "test*file", "test/path/file", true},

		// Real-world patterns
		{"node_modules", "node_modules/*", "node_modules/package.json", true},
		{"node_modules nested", "node_modules/*", "node_modules/lib/index.js", true},
		{"git directory", ".git/*", ".git/config", true},
		{"git objects", ".git/*", ".git/objects/abc123", true},
		{"hidden files", ".*", ".env", true},
		{"hidden files", ".*", ".gitignore", true},

		// Extensions
		{"all tmp files", "*.tmp", "file.tmp", true},
		{"all tmp files", "*.tmp", "path/to/file.tmp", true},
		{"specific extension", "*.js", "script.js", true},
		{"specific extension", "*.js", "script.ts", false},

		// Edge cases
		{"empty pattern", "", "", true},
		{"empty pattern no match", "", "something", false},
		{"literal brackets", "[", "[", true},
		{"unclosed bracket", "[abc", "[abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Match(tt.pattern, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	tests := []struct {
		pattern  string
		input    string
		expected bool
	}{
		// Verify that translate produces valid regex
		{"*", "anything", true},
		{"?", "x", true},
		{"[abc]", "b", true},
		{"[!xyz]", "a", true},
	}

	for _, tt := range tests {
		regex := Translate(tt.pattern)
		t.Logf("Pattern %q translated to %q", tt.pattern, regex)

		// Verify the regex compiles and works
		got, err := Match(tt.pattern, tt.input)
		if err != nil {
			t.Errorf("Pattern %q failed: %v", tt.pattern, err)
		}
		if got != tt.expected {
			t.Errorf("Pattern %q with input %q: got %v, want %v", tt.pattern, tt.input, got, tt.expected)
		}
	}
}

func TestFilter(t *testing.T) {
	names := []string{
		"file.txt",
		"file.tmp",
		"test.tmp",
		"data.json",
		".hidden",
		".git/config",
	}

	tests := []struct {
		pattern  string
		expected []string
	}{
		{"*.tmp", []string{"file.tmp", "test.tmp"}},
		{".*", []string{".hidden", ".git/config"}},
		{"*", names},          // all files
		{"?.txt", []string{}}, // no single char .txt files
		{"file.*", []string{"file.txt", "file.tmp"}},
	}

	for _, tt := range tests {
		got, err := Filter(names, tt.pattern)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(got) != len(tt.expected) {
			t.Errorf("Filter(%q): got %v, want %v", tt.pattern, got, tt.expected)
			continue
		}

		for i, name := range got {
			if name != tt.expected[i] {
				t.Errorf("Filter(%q): got %v, want %v", tt.pattern, got, tt.expected)
				break
			}
		}
	}
}

func TestFilterFalse(t *testing.T) {
	names := []string{
		"file.txt",
		"file.tmp",
		"test.tmp",
	}

	got, err := FilterFalse(names, "*.tmp")
	if err != nil {
		t.Fatalf("FilterFalse error: %v", err)
	}

	expected := []string{"file.txt"}
	if len(got) != 1 || got[0] != expected[0] {
		t.Errorf("FilterFalse: got %v, want %v", got, expected)
	}
}

// Benchmark to ensure performance with cache
func BenchmarkMatch(b *testing.B) {
	pattern := "node_modules/*"
	name := "node_modules/package.json"

	for i := 0; i < b.N; i++ {
		_, _ = Match(pattern, name)
	}
}

func BenchmarkMatchNoCache(b *testing.B) {
	name := "node_modules/package.json"

	for i := 0; i < b.N; i++ {
		pattern := "node_modules/*"
		// Clear cache to simulate no caching
		patternCache.Delete(pattern)
		_, _ = Match(pattern, name)
	}
}
