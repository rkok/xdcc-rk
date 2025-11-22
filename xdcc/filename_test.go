package xdcc

import "testing"

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

