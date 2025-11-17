package downloader

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

// SelectDownloadedFile finds the best downloaded file in workdir for the given video ID.
// It prefers common playable formats (mp4, mkv, webm, etc.).
func SelectDownloadedFile(workdir, id string) (string, error) {
	// Try to find files matching the ID
	candidates, err := filepath.Glob(filepath.Join(workdir, id+".*"))
	if err != nil {
		return "", err
	}

	if len(candidates) == 0 {
		// Fallback: try any file in workdir
		all, _ := filepath.Glob(filepath.Join(workdir, "*"))
		if len(all) == 0 {
			return "", errors.New("no output file found")
		}
		candidates = all
	}

	// Sort by extension priority
	sort.SliceStable(candidates, func(i, j int) bool {
		pri := extPriority(filepath.Ext(candidates[i]))
		prj := extPriority(filepath.Ext(candidates[j]))
		if pri == prj {
			return candidates[i] < candidates[j]
		}
		return pri < prj
	})

	return candidates[0], nil
}

// extPriority returns a priority score for file extensions (lower = better).
// Prefers common playable video formats.
func extPriority(ext string) int {
	ext = strings.ToLower(ext)
	switch ext {
	case ".mp4":
		return 0
	case ".mkv":
		return 1
	case ".webm":
		return 2
	case ".mov":
		return 3
	case ".avi":
		return 4
	case ".flv":
		return 5
	default:
		return 100
	}
}