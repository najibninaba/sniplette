package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MakeTempWorkdir creates a unique temp directory under $TMPDIR/ig2wa.
func MakeTempWorkdir(prefix string) (string, error) {
	base := filepath.Join(os.TempDir(), "ig2wa")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	// Prefix helps identification; OS will add random suffix.
	dir, err := os.MkdirTemp(base, prefix+"-")
	if err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureDir creates the directory path if it does not exist.
func EnsureDir(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	return os.MkdirAll(path, 0o755)
}

// RemoveIfExists deletes the file if present.
func RemoveIfExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	} else if os.IsNotExist(err) {
		return nil
	} else {
		return err
	}
}

// SanitizeFilename cleans a string to be safe as a filename:
// - Replace spaces with underscores
// - Replace forbidden characters with underscores
// - Trim duplicated underscores
// - Truncate to a reasonable length (~200 runes)
func SanitizeFilename(s string) string {
	if s == "" {
		return "untitled"
	}
	// Normalize spaces
	s = strings.ReplaceAll(s, " ", "_")
	// Replace forbidden characters
	forbidden := `[]/\:*?"<>|#%{}$!@+^~\` + "`" + `=&;`
	for _, r := range forbidden {
		s = strings.ReplaceAll(s, string(r), "_")
	}
	// Collapse runs of underscores
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	s = strings.Trim(s, "._-")

	// Truncate to 200 runes while preserving UTF-8 integrity
	const maxRunes = 200
	if utf8.RuneCountInString(s) > maxRunes {
		var b strings.Builder
		b.Grow(len(s))
		count := 0
		for _, r := range s {
			if count >= maxRunes {
				break
			}
			b.WriteRune(r)
			count++
		}
		s = b.String()
	}

	if s == "" {
		return "untitled"
	}
	return s
}

// WriteCaptionFile writes a .txt with the same basename as the given outputPath.
func WriteCaptionFile(outputPath string, content string) (string, error) {
	base := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
	captionPath := base + ".txt"
	if err := os.WriteFile(captionPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return captionPath, nil
}
