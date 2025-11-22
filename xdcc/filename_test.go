package xdcc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal filename unchanged",
			input:    "[HorribleSubs] Anime - 01.mkv",
			expected: "[HorribleSubs] Anime - 01.mkv",
		},
		{
			name:     "path traversal attack",
			input:    "../../../etc/passwd",
			expected: "_.._.._etc_passwd",
		},
		{
			name:     "shell injection attempt",
			input:    "file; rm -rf /",
			expected: "file_ rm -rf _",
		},
		{
			name:     "chinese characters",
			input:    "中文文件名.txt",
			expected: "_.txt",
		},
		{
			name:     "accented characters normalized",
			input:    "café_résumé.pdf",
			expected: "cafe_resume.pdf",
		},
		{
			name:     "null byte and control characters",
			input:    "file\x00name\n.txt",
			expected: "file_name_.txt",
		},
		{
			name:     "html/script tags",
			input:    "<script>alert(1)</script>.html",
			expected: "_script_alert(1)_script_.html",
		},
		{
			name:     "windows problematic characters",
			input:    "file:name*?.txt",
			expected: "file_name_.txt",
		},
		{
			name:     "brackets allowed",
			input:    "[Group] File [1080p].mkv",
			expected: "[Group] File [1080p].mkv",
		},
		{
			name:     "parentheses allowed",
			input:    "File (2024).mp4",
			expected: "File (2024).mp4",
		},
		{
			name:     "at symbol and comma allowed",
			input:    "user@host,file.txt",
			expected: "user@host,file.txt",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "unnamed_file",
		},
		{
			name:     "only invalid characters",
			input:    "///\\\\\\",
			expected: "unnamed_file",
		},
		{
			name:     "multiple consecutive underscores collapsed",
			input:    "file___name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "leading and trailing dots removed",
			input:    "...file.txt...",
			expected: "file.txt",
		},
		{
			name:     "leading and trailing spaces removed",
			input:    "   file.txt   ",
			expected: "file.txt",
		},
		{
			name:     "emoji characters",
			input:    "file😀name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "mixed valid and invalid",
			input:    "File (2024) [1080p] - Episode 01.mkv",
			expected: "File (2024) [1080p] - Episode 01.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetUniqueFilePath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "xdcc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: File doesn't exist - should return original path
	testPath := filepath.Join(tmpDir, "testfile.mp3")
	result := GetUniqueFilePath(testPath)
	if result != testPath {
		t.Errorf("Expected %s, got %s", testPath, result)
	}

	// Test 2: File exists - should return path with -1 suffix
	// Create the file
	if err := os.WriteFile(testPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "testfile-1.mp3")
	result = GetUniqueFilePath(testPath)
	if result != expectedPath {
		t.Errorf("Expected %s, got %s", expectedPath, result)
	}

	// Test 3: Both original and -1 exist - should return -2
	if err := os.WriteFile(expectedPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	expectedPath2 := filepath.Join(tmpDir, "testfile-2.mp3")
	result = GetUniqueFilePath(testPath)
	if result != expectedPath2 {
		t.Errorf("Expected %s, got %s", expectedPath2, result)
	}

	// Test 4: File without extension
	testPathNoExt := filepath.Join(tmpDir, "README")
	if err := os.WriteFile(testPathNoExt, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	expectedNoExt := filepath.Join(tmpDir, "README-1")
	result = GetUniqueFilePath(testPathNoExt)
	if result != expectedNoExt {
		t.Errorf("Expected %s, got %s", expectedNoExt, result)
	}

	// Test 5: File with multiple dots in name
	testPathMultiDot := filepath.Join(tmpDir, "archive.tar.gz")
	if err := os.WriteFile(testPathMultiDot, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	expectedMultiDot := filepath.Join(tmpDir, "archive.tar-1.gz")
	result = GetUniqueFilePath(testPathMultiDot)
	if result != expectedMultiDot {
		t.Errorf("Expected %s, got %s", expectedMultiDot, result)
	}
}
