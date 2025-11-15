package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ig2wa/internal/model"
	"ig2wa/internal/util"
)

// Options controls downloader behavior.
type Options struct {
	DownloaderPath string // Path to yt-dlp or youtube-dl
	Verbose        bool
	KeepTemp       bool // Reserved for future; cleanup handled by caller
	MetadataOnly   bool // If true, only fetch metadata; do not download the media file
}

// Download fetches metadata (and optionally downloads the media) for a given URL.
// Returns the DownloadedVideo and the temp workdir used (for caller to cleanup).
func Download(ctx context.Context, url string, opts Options) (model.DownloadedVideo, string, error) {
	if opts.DownloaderPath == "" {
		return model.DownloadedVideo{}, "", errors.New("downloader path is required")
	}

	workdir, err := util.MakeTempWorkdir("job")
	if err != nil {
		return model.DownloadedVideo{}, "", fmt.Errorf("create temp dir: %w", err)
	}

	// First: get metadata as JSON
	info, err := fetchMetadata(ctx, opts, url)
	if err != nil {
		return model.DownloadedVideo{}, workdir, err
	}

	// If only metadata is needed (dry-run), return early with no InputPath
	if opts.MetadataOnly {
		return model.DownloadedVideo{
			InputPath:   "",
			DurationSec: info.Duration,
			Title:       info.Title,
			Uploader:    info.Uploader,
			ID:          info.ID,
			Description: info.Description,
			Width:       info.Width,
			Height:      info.Height,
			URL:         url,
		}, workdir, nil
	}

	// Download best available file into workdir
	// Use a fixed template based on ID to know where the file lands.
	outTemplate := filepath.Join(workdir, "%(id)s.%(ext)s")
	args := []string{
		"-f", "bestvideo+bestaudio/best",
		"-o", outTemplate,
		"--no-playlist",
		url,
	}
	_, runErr := util.Run(ctx, util.CmdSpec{
		Path:    opts.DownloaderPath,
		Args:    args,
		Dir:     workdir,
		Verbose: opts.Verbose,
	})
	if runErr != nil {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("downloader failed: %w", runErr)
	}

	// Resolve actual downloaded path(s)
	candidates, globErr := filepath.Glob(filepath.Join(workdir, info.ID+".*"))
	if globErr != nil {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("resolve download: %w", globErr)
	}
	if len(candidates) == 0 {
		// fallback: try find any file in workdir
		all, _ := filepath.Glob(filepath.Join(workdir, "*"))
		if len(all) == 0 {
			return model.DownloadedVideo{}, workdir, errors.New("download succeeded but no output file found")
		}
		candidates = all
	}

	// Prefer common playable containers/extensions
	sort.SliceStable(candidates, func(i, j int) bool {
		pri := extPriority(filepath.Ext(candidates[i]))
		prj := extPriority(filepath.Ext(candidates[j]))
		if pri == prj {
			return candidates[i] < candidates[j]
		}
		return pri < prj
	})
	input := candidates[0]

	return model.DownloadedVideo{
		InputPath:   input,
		DurationSec: info.Duration,
		Title:       info.Title,
		Uploader:    info.Uploader,
		ID:          info.ID,
		Description: info.Description,
		Width:       info.Width,
		Height:      info.Height,
		URL:         url,
	}, workdir, nil
}

func fetchMetadata(ctx context.Context, opts Options, url string) (YTDLPInfo, error) {
	args := []string{
		"--dump-json",
		"-f", "bestvideo+bestaudio/best",
		"--no-playlist",
		url,
	}
	res, runErr := util.Run(ctx, util.CmdSpec{
		Path:    opts.DownloaderPath,
		Args:    args,
		Verbose: opts.Verbose,
	})
	if runErr != nil && len(res.Stdout) == 0 {
		return YTDLPInfo{}, fmt.Errorf("metadata fetch failed: %w", runErr)
	}

	// yt-dlp sometimes prints progress/info to stderr but JSON to stdout
	// Parse the last JSON object if multiple lines exist.
	data := strings.TrimSpace(string(res.Stdout))
	dec := json.NewDecoder(strings.NewReader(data))
	var info YTDLPInfo
	if err := dec.Decode(&info); err != nil {
		// Try to recover if stdout contains multiple JSON objects by scanning lines
		var lastErr error = err
		lines := strings.Split(data, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			var tmp YTDLPInfo
			if json.Unmarshal([]byte(line), &tmp) == nil && tmp.ID != "" {
				info = tmp
				lastErr = nil
				break
			}
		}
		if lastErr != nil {
			return YTDLPInfo{}, fmt.Errorf("parse metadata JSON: %w", lastErr)
		}
	}
	return info, nil
}

func extPriority(ext string) int {
	e := strings.ToLower(strings.TrimPrefix(ext, "."))
	switch e {
	case "mp4":
		return 0
	case "mkv":
		return 1
	case "webm":
		return 2
	case "mov":
		return 3
	default:
		return 9
	}
}

// CleanupWorkdir removes the given temp workdir (best-effort).
// Not strictly required but useful if a caller wants explicit cleanup here.
func CleanupWorkdir(dir string) {
	_ = os.RemoveAll(dir)
}
