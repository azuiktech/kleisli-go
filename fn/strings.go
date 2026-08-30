package fn

import "strings"

// IsSpace reports whether s is empty or contains only whitespace characters.
// Mirrors Go's unicode.IsSpace convention.
func IsSpace(s string) bool { return strings.TrimSpace(s) == "" }

// DefaultIfEmpty returns fallback when s is empty, otherwise s.
func DefaultIfEmpty(fallback string) func(string) string {
	return func(s string) string {
		if s == "" {
			return fallback
		}
		return s
	}
}

// DefaultIfSpace returns fallback when s is empty or all whitespace, otherwise s.
func DefaultIfSpace(fallback string) func(string) string {
	return func(s string) string {
		if IsSpace(s) {
			return fallback
		}
		return s
	}
}

// HasPrefix returns a predicate that reports whether s has the given prefix.
func HasPrefix(prefix string) func(string) bool {
	return func(s string) bool { return strings.HasPrefix(s, prefix) }
}

// HasSuffix returns a predicate that reports whether s has the given suffix.
func HasSuffix(suffix string) func(string) bool {
	return func(s string) bool { return strings.HasSuffix(s, suffix) }
}

// Contains returns a predicate that reports whether s contains sub.
func Contains(sub string) func(string) bool {
	return func(s string) bool { return strings.Contains(s, sub) }
}

// ContainsAny returns a predicate that reports whether s contains any Unicode
// code point in chars.
func ContainsAny(chars string) func(string) bool {
	return func(s string) bool { return strings.ContainsAny(s, chars) }
}

// EqualFold returns a predicate that reports whether s equals t under
// Unicode case-folding (case-insensitive equality).
func EqualFold(t string) func(string) bool {
	return func(s string) bool { return strings.EqualFold(s, t) }
}

// TrimPrefix returns a transform that removes prefix from s if present.
func TrimPrefix(prefix string) func(string) string {
	return func(s string) string { return strings.TrimPrefix(s, prefix) }
}

// TrimSuffix returns a transform that removes suffix from s if present.
func TrimSuffix(suffix string) func(string) string {
	return func(s string) string { return strings.TrimSuffix(s, suffix) }
}

// ReplaceAll returns a transform that replaces all occurrences of old with new.
func ReplaceAll(old, new string) func(string) string {
	return func(s string) string { return strings.ReplaceAll(s, old, new) }
}

// Split returns a transform that splits s by sep into a slice of substrings.
func Split(sep string) func(string) []string {
	return func(s string) []string { return strings.Split(s, sep) }
}

// Join returns a transform that joins a slice of strings with sep.
func Join(sep string) func([]string) string {
	return func(ss []string) string { return strings.Join(ss, sep) }
}

// Trim returns a transform that removes all leading and trailing Unicode code
// points contained in cutset.
func Trim(cutset string) func(string) string {
	return func(s string) string { return strings.Trim(s, cutset) }
}

// Count returns a transform that counts non-overlapping occurrences of sub in s.
func Count(sub string) func(string) int {
	return func(s string) int { return strings.Count(s, sub) }
}

// Truncate returns a transform that shortens s to at most maxLen runes.
// Counts runes, not bytes, so multi-byte characters are handled correctly.
func Truncate(maxLen int) func(string) string {
	return func(s string) string {
		if len([]rune(s)) <= maxLen {
			return s
		}
		return string([]rune(s)[:maxLen])
	}
}
