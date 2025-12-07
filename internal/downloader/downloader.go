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

var ErrAuthRequired = errors.New("authentication required")
var ErrLuxFailed = errors.New("lux backend failed")

// Options controls downloader behavior.
type Options struct {
	DownloaderPath string // Path to yt-dlp or youtube-dl
	Verbose        bool
	KeepTemp       bool // Reserved for future; cleanup handled by caller
	MetadataOnly   bool // If true, only fetch metadata; do not download the media file

	// Progress reporting (optional)
	Reporter progress.Reporter
	JobID    string

	// Authentication
	CookiesFromBrowser string // e.g., "brave", "chrome:Default", "firefox"

	// NoFallback disables Lux fallback for TikTok/Twitter (use yt-dlp only)
	NoFallback bool
}

// appendAuthArgs adds authentication arguments to yt-dlp command.
func appendAuthArgs(args []string, opts Options) []string {
	if opts.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	return args
}

// Download fetches metadata (and optionally downloads the media) for a given URL.
// Returns the DownloadedVideo and the temp workdir used (for caller to cleanup).
//
// Platform dispatch:
//   - Instagram, YouTube: yt-dlp only
//   - Threads, Facebook: Lux only (yt-dlp has no extractor)
//   - TikTok, Twitter: yt-dlp primary, Lux fallback on failure
func Download(ctx context.Context, url string, opts Options) (model.DownloadedVideo, string, error) {
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

	// Detect platform for dispatch
	platform, _, perr := util.DetectPlatform(url)
	if perr != nil {
		return model.DownloadedVideo{}, workdir, perr
	}

	// Dispatch based on platform
	switch platform {
	case util.PlatformInstagram, util.PlatformYouTube:
		// yt-dlp only
		if opts.DownloaderPath == "" {
			return model.DownloadedVideo{}, workdir, errors.New("downloader path is required")
		}
		return downloadViaYtDlp(ctx, url, workdir, opts, platform)

	case util.PlatformThreads, util.PlatformFacebook:
		// Lux only (yt-dlp has no extractor for these)
		return downloadViaLux(ctx, url, workdir, opts, platform)

	case util.PlatformTikTok, util.PlatformTwitter:
		// Hybrid: try yt-dlp first, fallback to Lux
		if opts.DownloaderPath == "" {
			// No yt-dlp available, go directly to Lux
			return downloadViaLux(ctx, url, workdir, opts, platform)
		}
		dv, _, ytErr := downloadViaYtDlp(ctx, url, workdir, opts, platform)
		if ytErr == nil {
			return dv, workdir, nil
		}
		// Check if we should fallback to Lux
		if opts.NoFallback || !shouldFallbackToLux(ytErr) {
			return dv, workdir, ytErr
		}
		// Fallback to Lux
		if opts.Reporter != nil {
			opts.Reporter.Update(progress.Update{
				JobID:   opts.JobID,
				Stage:   progress.StageMetadata,
				Percent: -1,
				Message: "Falling back to Lux backend",
			})
		}
		return downloadViaLux(ctx, url, workdir, opts, platform)

	default:
		return model.DownloadedVideo{}, workdir, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// shouldFallbackToLux determines if a yt-dlp error warrants falling back to Lux.
func shouldFallbackToLux(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())

	// Don't fallback on auth/user errors - user needs to fix these
	if strings.Contains(low, "login required") ||
		strings.Contains(low, "cookies-from-browser") ||
		strings.Contains(low, "authentication required") ||
		errors.Is(err, ErrAuthRequired) {
		return false
	}

	// Don't fallback on context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Fallback on extractor/format failures
	if strings.Contains(low, "unsupported url") ||
		strings.Contains(low, "no video formats found") ||
		strings.Contains(low, "extractor error") ||
		strings.Contains(low, "unable to extract") {
		return true
	}

	return false
}

// downloadViaYtDlp uses yt-dlp to download video.
func downloadViaYtDlp(ctx context.Context, url, workdir string, opts Options, platform util.Platform) (model.DownloadedVideo, string, error) {
	// Normalize URL for yt-dlp
	normURL := util.NormalizeURL(url, platform)

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

	// Check if Instagram to conditionally skip --no-playlist
	// (Instagram stories are treated as playlists by yt-dlp)
	isInstagram := platform == util.PlatformInstagram

	// Auth args must come first for yt-dlp compatibility
	args := appendAuthArgs([]string{}, opts)
	args = append(args,
		"-f", "bestvideo+bestaudio/best",
		"-o", outTemplate,
	)
	// Skip --no-playlist for Instagram as stories are treated as playlists
	if !isInstagram {
		args = append(args, "--no-playlist")
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
		Path:          opts.DownloaderPath,
		Args:          args,
		Dir:           workdir,
		Verbose:       opts.Verbose && opts.Reporter == nil,
		SensitiveArgs: []string{"--cookies", "--cookies-from-browser"},
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
		// Check for auth-related errors
		low := strings.ToLower(runErr.Error())
		if strings.Contains(low, "login required") ||
			strings.Contains(low, "provide cookies") ||
			strings.Contains(low, "cookies-from-browser") ||
			strings.Contains(low, "403") {
			return model.DownloadedVideo{}, workdir, fmt.Errorf("%w: try --cookies-from-browser brave (or chrome, firefox, safari)", ErrAuthRequired)
		}
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
	// Auth args must come first for yt-dlp compatibility
	// Note: --no-playlist is omitted for metadata as it breaks Instagram stories
	args := appendAuthArgs([]string{}, opts)
	args = append(args, "--dump-json", url)
	res, runErr := util.Run(ctx, util.CmdSpec{
		Path:          opts.DownloaderPath,
		Args:          args,
		Verbose:       opts.Verbose && opts.Reporter == nil,
		SensitiveArgs: []string{"--cookies", "--cookies-from-browser"},
		// Forward stderr lines to Reporter logs in verbose UI mode (optional)
		StderrLine: func(line string) {
			if opts.Reporter != nil && opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStderr, Line: line})
			}
		},
	})
	if runErr != nil && len(res.Stdout) == 0 {
		msg := strings.ToLower(runErr.Error())
		stderr := strings.ToLower(string(res.Stderr))
		combined := msg + "\n" + stderr
		// Check for authentication-related errors
		if strings.Contains(combined, "login required") ||
			strings.Contains(combined, "provide cookies") ||
			strings.Contains(combined, "cookies-from-browser") ||
			(strings.Contains(combined, "403") && strings.Contains(combined, "instagram")) {
			return YTDLPInfo{}, fmt.Errorf("%w: try --cookies-from-browser brave (or chrome, firefox, safari)", ErrAuthRequired)
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
