package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ig2wa/internal/model"
	"ig2wa/internal/progress"
	"ig2wa/internal/util"
)

var ErrThreadsUnsupported = errors.New("threads not supported (yt-dlp has no extractor)")

func isThreadsURL(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.Index(s, "://"); i != -1 {
		s = s[i+3:]
	}
	s = strings.TrimPrefix(s, "www.")
	return strings.HasPrefix(s, "threads.net/") || strings.HasPrefix(s, "threads.com/")
}

// Options controls downloader behavior.
type Options struct {
	DownloaderPath string // Path to yt-dlp or youtube-dl
	Verbose        bool
	KeepTemp       bool // Reserved for future; cleanup handled by caller
	MetadataOnly   bool // If true, only fetch metadata; do not download the media file

	// Progress reporting (optional)
	Reporter progress.Reporter
	JobID    string
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

	if opts.Reporter != nil {
		opts.Reporter.Update(progress.Update{
			JobID:   opts.JobID,
			Stage:   progress.StageMetadata,
			Percent: -1,
			Message: "Fetching metadata",
		})
	}

	// Fail fast for Threads URLs (unsupported upstream by yt-dlp)
	if isThreadsURL(url) {
		return model.DownloadedVideo{}, workdir, ErrThreadsUnsupported
	}

	// Normalize URL for yt-dlp (e.g., threads.com -> threads.net for Threads)
	normURL := url
	if pl, _, derr := util.DetectPlatform(url); derr == nil {
		normURL = util.NormalizeURL(url, pl)
	}

	// First: get metadata as JSON
	info, err := fetchMetadata(ctx, opts, normURL)
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
	}
	if opts.Reporter != nil {
		args = append(args, "--newline")
	}
	args = append(args, normURL)

	if opts.Reporter != nil {
		opts.Reporter.Update(progress.Update{
			JobID:   opts.JobID,
			Stage:   progress.StageDownloading,
			Percent: 0,
			Message: "Starting download",
		})
	}

	_, runErr := util.Run(ctx, util.CmdSpec{
		Path:    opts.DownloaderPath,
		Args:    args,
		Dir:     workdir,
		Verbose: opts.Verbose && opts.Reporter == nil,
		StdoutLine: func(line string) {
			if opts.Reporter == nil {
				return
			}
			// Forward raw logs in verbose mode
			if opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStdout, Line: line})
			}
			// Try to parse progress lines (yt-dlp --newline commonly writes progress to stdout)
			if u, ok := parseYTDLPProgress(line, opts.JobID); ok {
				opts.Reporter.Update(u)
			}
		},
		StderrLine: func(line string) {
			if opts.Reporter == nil {
				return
			}
			// Forward raw logs in verbose mode
			if opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStderr, Line: line})
			}
			// Try to parse progress lines
			if u, ok := parseYTDLPProgress(line, opts.JobID); ok {
				opts.Reporter.Update(u)
			}
		},
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
	// Normalize URL for yt-dlp compatibility
	normURL := url
	if pl, _, err := util.DetectPlatform(url); err == nil {
		normURL = util.NormalizeURL(url, pl)
	}

	// Fail fast for Threads URLs (unsupported upstream by yt-dlp)
	if isThreadsURL(url) || isThreadsURL(normURL) {
		return YTDLPInfo{}, ErrThreadsUnsupported
	}

	args := []string{
		"--dump-json",
		"-f", "bestvideo+bestaudio/best",
		"--no-playlist",
		normURL,
	}
	res, runErr := util.Run(ctx, util.CmdSpec{
		Path:    opts.DownloaderPath,
		Args:    args,
		Verbose: opts.Verbose && opts.Reporter == nil,
		// Forward stderr lines to Reporter logs in verbose UI mode (optional)
		StderrLine: func(line string) {
			if opts.Reporter != nil && opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStderr, Line: line})
			}
		},
	})
	if runErr != nil && len(res.Stdout) == 0 {
		msg := strings.ToLower(runErr.Error())
		if strings.Contains(msg, "unsupported url") && (strings.Contains(msg, "threads.net") || strings.Contains(msg, "threads.com")) {
			return YTDLPInfo{}, ErrThreadsUnsupported
		}
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
			if err := json.Unmarshal([]byte(line), &tmp); err == nil && tmp.ID != "" {
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

func parseYTDLPProgress(line, jobID string) (u progress.Update, ok bool) {
	u = progress.Update{
		JobID:   jobID,
		Percent: -1,
		Message: "",
		Stage:   progress.StageDownloading,
	}
	if strings.Contains(line, "[download]") {
		u.Message = "Downloading"
		// crude percent parsing: find first token containing '%'
		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.Contains(f, "%") {
				p := strings.TrimSuffix(strings.TrimSpace(f), "%")
				if p != "" {
					if v, err := strconv.ParseFloat(strings.TrimSpace(p), 64); err == nil {
						u.Percent = v
						break
					}
				}
			}
		}
		// speed: look for " at <speed>" pattern
		if i := strings.Index(line, " at "); i != -1 {
			rest := strings.TrimSpace(line[i+4:])
			if rest != "" {
				sp := strings.Fields(rest)
				if len(sp) > 0 {
					speed := sp[0]
					u.Speed = &speed
				}
			}
		}
		// ETA parsing
		if j := strings.Index(line, " ETA "); j != -1 {
			rest := strings.TrimSpace(line[j+5:])
			if rest != "" {
				token := strings.Fields(rest)
				if len(token) > 0 {
					if d, err := parseETA(token[0]); err == nil {
						u.ETA = &d
					}
				}
			}
		}
		return u, true
	}
	if strings.Contains(line, "Merging formats") || strings.Contains(line, "[Merger]") {
		u.Stage = progress.StageMerging
		u.Message = "Merging"
		u.Percent = -1
		return u, true
	}
	return u, false
}

func parseETA(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) == 2 {
		// mm:ss
		min, err1 := strconv.Atoi(parts[0])
		sec, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid ETA %q", s)
		}
		return time.Duration(min)*time.Minute + time.Duration(sec)*time.Second, nil
	}
	if len(parts) == 3 {
		// hh:mm:ss
		hr, err1 := strconv.Atoi(parts[0])
		min, err2 := strconv.Atoi(parts[1])
		sec, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, fmt.Errorf("invalid ETA %q", s)
		}
		return time.Duration(hr)*time.Hour + time.Duration(min)*time.Minute + time.Duration(sec)*time.Second, nil
	}
	return 0, fmt.Errorf("invalid ETA %q", s)
}
