package xdcc

import (
	"fmt"
	"os"
	"path/filepath"
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

// GetUniqueFilePath returns a unique file path by adding a numeric suffix if the file exists.
// Examples:
//   file.mp3 -> file.mp3 (if doesn't exist)
//   file.mp3 -> file-1.mp3 (if file.mp3 exists)
//   file.mp3 -> file-2.mp3 (if file.mp3 and file-1.mp3 exist)
func GetUniqueFilePath(basePath string) string {
	// Check if the original path is available
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return basePath
	}

	// Split into directory, name, and extension
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// Try incrementing suffixes until we find an available name
	for i := 1; ; i++ {
		newPath := filepath.Join(dir, fmt.Sprintf("%s-%d%s", nameWithoutExt, i, ext))
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}
