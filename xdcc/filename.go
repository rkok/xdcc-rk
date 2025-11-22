package xdcc

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// SanitizeFilename converts a filename to ASCII-only safe characters
// Matches the pattern: /^[\w,\s()@.\[\]-]+$/
// - Normalizes Unicode to decomposed form (NFD)
// - Transliterates common characters where possible
// - Replaces invalid characters with underscores
// - Handles multibyte characters safely
func SanitizeFilename(filename string) string {
	if filename == "" {
		return "unnamed_file"
	}

	// Step 1: Normalize Unicode to NFD (decomposed form)
	// This separates accented characters into base + combining marks
	// e.g., "é" becomes "e" + combining acute accent
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalized, _, _ := transform.String(t, filename)

	// Step 2: Build sanitized filename character by character
	var result strings.Builder
	result.Grow(len(normalized))

	for _, r := range normalized {
		// Allow: alphanumeric, underscore, comma, space, parentheses, @, dot, hyphen, brackets
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == ',' || r == ' ' ||
			r == '(' || r == ')' || r == '@' ||
			r == '.' || r == '-' || r == '[' || r == ']' {
			result.WriteRune(r)
		} else {
			// Replace any other character (including multibyte) with underscore
			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	// Step 3: Clean up edge cases
	// - Trim leading/trailing spaces and dots (filesystem issues)
	sanitized = strings.Trim(sanitized, " .")

	// - Collapse multiple consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	// - Ensure we have something left (not just underscores)
	if sanitized == "" || sanitized == "_" || strings.Trim(sanitized, "_") == "" {
		return "unnamed_file"
	}

	return sanitized
}

