package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectDownloadedFile(t *testing.T) {
	// Create temp directory for tests
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		files     []string // Files to create in tmpDir
		videoID   string
		wantFile  string // Expected basename
		wantError bool
	}{
		{
			name:     "prefers mp4 over webm",
			files:    []string{"abc123.webm", "abc123.mp4"},
			videoID:  "abc123",
			wantFile: "abc123.mp4",
		},
		{
			name:     "prefers mp4 over mkv",
			files:    []string{"abc123.mkv", "abc123.mp4", "abc123.webm"},
			videoID:  "abc123",
			wantFile: "abc123.mp4",
		},
		{
			name:     "mkv when no mp4",
			files:    []string{"abc123.mkv", "abc123.webm"},
			videoID:  "abc123",
			wantFile: "abc123.mkv",
		},
		{
			name:     "webm when no better format",
			files:    []string{"abc123.webm"},
			videoID:  "abc123",
			wantFile: "abc123.webm",
		},
		{
			name:     "fallback to any file when ID mismatch",
			files:    []string{"different.mp4"},
			videoID:  "abc123",
			wantFile: "different.mp4",
		},
		{
			name:      "error when no files",
			files:     []string{},
			videoID:   "abc123",
			wantError: true,
		},
		{
			name:     "handles multiple extensions correctly",
			files:    []string{"abc123.avi", "abc123.flv", "abc123.mov"},
			videoID:  "abc123",
			wantFile: "abc123.mov", // mov priority (3) < avi (4) < flv (5)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0o755); err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}
			defer os.RemoveAll(testDir)

			// Create test files
			for _, f := range tt.files {
				path := filepath.Join(testDir, f)
				if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", f, err)
				}
			}

			// Run test
			got, err := SelectDownloadedFile(testDir, tt.videoID)

			if tt.wantError {
				if err == nil {
					t.Errorf("SelectDownloadedFile() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SelectDownloadedFile() unexpected error: %v", err)
				return
			}

			gotBase := filepath.Base(got)
			if gotBase != tt.wantFile {
				t.Errorf("SelectDownloadedFile() = %v, want %v", gotBase, tt.wantFile)
			}
		})
	}
}

func TestExtPriority(t *testing.T) {
	tests := []struct {
		ext  string
		want int
	}{
		{ext: ".mp4", want: 0},
		{ext: ".mkv", want: 1},
		{ext: ".webm", want: 2},
		{ext: ".mov", want: 3},
		{ext: ".avi", want: 4},
		{ext: ".flv", want: 5},
		{ext: ".unknown", want: 100},
		{ext: ".MP4", want: 0}, // Case insensitive
		{ext: ".MKV", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := extPriority(tt.ext)
			if got != tt.want {
				t.Errorf("extPriority(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}