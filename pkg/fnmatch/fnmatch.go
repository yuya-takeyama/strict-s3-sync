// Package fnmatch provides Unix shell style pattern matching compatible with Python's fnmatch module.
//
// This implementation is based on Python's fnmatch module from the CPython repository.
// Original source: https://github.com/python/cpython/blob/main/Lib/fnmatch.py
//
// Copyright (c) 2001-2024 Python Software Foundation.
// All Rights Reserved.
//
// This Go port is licensed under the MIT License, but includes code derived from
// Python's fnmatch module which is licensed under the Python Software Foundation License Version 2.
//
// Patterns are Unix shell style:
//
//   - matches everything (including path separators)
//     ?       matches any single character
//     [seq]   matches any character in seq
//     [!seq]  matches any character not in seq
//
// Unlike filesystem glob patterns, * matches path separators (like Python's fnmatch).
// This behavior is compatible with AWS S3 sync's exclude patterns.
package fnmatch

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// patternCache caches compiled regular expressions for performance
var patternCache = sync.Map{}

// Match tests whether name matches the shell pattern.
// The pattern matching is case-sensitive.
func Match(pattern, name string) (bool, error) {
	re, err := compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(name), nil
}

// compile converts a shell pattern to a compiled regular expression,
// using a cache for performance.
func compile(pattern string) (*regexp.Regexp, error) {
	if cached, ok := patternCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}

	translated := Translate(pattern)
	re, err := regexp.Compile(translated)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern %q: %w", pattern, err)
	}

	patternCache.Store(pattern, re)
	return re, nil
}

// Translate converts a shell pattern to a regular expression string.
// This function is exported for compatibility and testing purposes.
func Translate(pattern string) string {
	var result strings.Builder
	result.WriteString("(?s:^") // (?s:...) makes . match newlines, ^ anchors to start

	i := 0
	n := len(pattern)

	for i < n {
		c := pattern[i]
		i++

		switch c {
		case '*':
			// Compress consecutive * into one
			for i < n && pattern[i] == '*' {
				i++
			}
			result.WriteString(".*")

		case '?':
			result.WriteByte('.')

		case '[':
			j := i
			// Check for negation
			if j < n && pattern[j] == '!' {
				j++
			}
			// Check for closing bracket as first character
			if j < n && pattern[j] == ']' {
				j++
			}
			// Find the closing bracket
			for j < n && pattern[j] != ']' {
				j++
			}

			if j >= n {
				// No closing bracket found, treat [ as literal
				result.WriteString("\\[")
			} else {
				stuff := pattern[i:j]
				i = j + 1

				if len(stuff) == 0 {
					// Empty range: never match
					result.WriteString("(?!)")
				} else if stuff == "!" {
					// Negated empty range: match any character
					result.WriteByte('.')
				} else {
					// Build character class
					result.WriteByte('[')
					if stuff[0] == '!' {
						result.WriteByte('^')
						stuff = stuff[1:]
					}
					// Escape special characters in character class
					stuff = escapeForCharClass(stuff)
					result.WriteString(stuff)
					result.WriteByte(']')
				}
			}

		default:
			// Escape special regex characters
			result.WriteString(regexp.QuoteMeta(string(c)))
		}
	}

	result.WriteString("$)") // $ anchors to end
	return result.String()
}

// escapeForCharClass escapes special characters within a character class
func escapeForCharClass(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', ']':
			result.WriteByte('\\')
			result.WriteByte(c)
		default:
			// Don't escape hyphens - they're needed for ranges
			result.WriteByte(c)
		}
	}
	return result.String()
}

// Filter returns a list of names that match the pattern.
func Filter(names []string, pattern string) ([]string, error) {
	re, err := compile(pattern)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, name := range names {
		if re.MatchString(name) {
			result = append(result, name)
		}
	}
	return result, nil
}

// FilterFalse returns a list of names that do not match the pattern.
func FilterFalse(names []string, pattern string) ([]string, error) {
	re, err := compile(pattern)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, name := range names {
		if !re.MatchString(name) {
			result = append(result, name)
		}
	}
	return result, nil
}
