package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ig2wa/internal/cli"
	"ig2wa/internal/downloader"
	"ig2wa/internal/encoder"
	"ig2wa/internal/model"
	"ig2wa/internal/util"
	"ig2wa/internal/util/deps"
	"ig2wa/internal/util/media"
	"ig2wa/internal/pipeline"

	"ig2wa/internal/ui"
	"golang.org/x/term"
)

const (
	ExitOK             = 0
	ExitCLIError       = 1
	ExitMissingDep     = 2
	ExitDownloadError  = 3
	ExitTranscodeError = 4
)

func main() {
	ctx := context.Background()

	parsed, err := cli.ParseFlags(ctx, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitCLIError)
	}

	// UI path: enabled when stdout is a TTY and not explicitly disabled.
	if !parsed.Options.NoUI && isTerminal() {
		// Ensure output directory exists before launching TUI
		if err := util.EnsureDir(parsed.Options.OutDir); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create output dir: %v\n", err)
			os.Exit(ExitCLIError)
		}
		if err := ui.Run(ctx, parsed.URLs, parsed.Options); err != nil {
			// UI returns an error if any job failed; print and exit non-zero
			fmt.Fprintln(os.Stderr, err)
			os.Exit(ExitCLIError)
		}
		os.Exit(ExitOK)
	}

	// Non-UI fallback (legacy textual flow)
	// Dependency detection
	downloaderPath, derr := deps.FindDownloader(parsed.Options.DLBinary)
	if derr != nil {
		fmt.Fprintln(os.Stderr, derr)
		os.Exit(ExitMissingDep)
	}
	ffmpegPath, ferr := deps.FindFFmpeg()
	if ferr != nil {
		fmt.Fprintln(os.Stderr, ferr)
		os.Exit(ExitMissingDep)
	}

	// Ensure output directory exists
	if err := util.EnsureDir(parsed.Options.OutDir); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output dir: %v\n", err)
		os.Exit(ExitCLIError)
	}

	// Process URLs sequentially
	for _, rawURL := range parsed.URLs {
		if err := processOne(ctx, rawURL, parsed, downloaderPath, ffmpegPath); err != nil {
			// Map error type to exit code
			code := ExitCLIError
			if errors.Is(err, errDownload) {
				code = ExitDownloadError
			} else if errors.Is(err, errEncode) {
				code = ExitTranscodeError
			} else if errors.Is(err, errMissingDep) {
				code = ExitMissingDep
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(code)
		}
	}

	os.Exit(ExitOK)
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

var (
	errMissingDep = errors.New("missing dependency")
	errDownload   = errors.New("download failed")
	errEncode     = errors.New("encode failed")
)

func processOne(ctx context.Context, rawURL string, parsed cli.Parsed, dlPath, ffmpegPath string) error {
	// Prepare job-level options
	metaOnly := parsed.Options.DryRun
	dv, tempDir, derr := downloader.Download(ctx, rawURL, downloader.Options{
		DownloaderPath: dlPath,
		Verbose:        parsed.Options.Verbose,
		KeepTemp:       parsed.Options.KeepTemp,
		MetadataOnly:   metaOnly,
	})
	// Always attempt cleanup at the end unless user asked to keep temp
	defer func() {
		if !parsed.Options.KeepTemp && tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if derr != nil {
		return fmt.Errorf("%w: %v", errDownload, derr)
	}

	// Plan encoding
	targetLongSide, crf := pipeline.PlanResolutionAndCRF(parsed.Options, dv, parsed.PresetCRF)
	encOpts := model.EncodeOptions{
		LongSidePx:       targetLongSide,
		ModeCRF:          parsed.Options.MaxSizeMB == 0 || dv.DurationSec <= 0 || parsed.Options.AudioOnly,
		CRF:              crf,
		MaxSizeMB:        parsed.Options.MaxSizeMB,
		AudioBitrateKbps: 96,
		VideoMinKbps:     500,
		VideoMaxKbps:     8000,
		Preset:           "veryfast",
		Profile:          "main",
		AudioOnly:        parsed.Options.AudioOnly,
		KeyInt:           48,
	}

	// Build output filename
	base := media.OutputBasename(dv, targetLongSide, parsed.Options.MaxSizeMB, encOpts)
	ext := ".mp4"
	if parsed.Options.AudioOnly {
		ext = ".m4a"
	}
	outputPath := filepath.Join(parsed.Options.OutDir, base+ext)

	if parsed.Options.DryRun {
		printPlan(rawURL, dlPath, ffmpegPath, tempDir, outputPath, dv, encOpts, parsed.Options)
		// No caption or encode in dry-run
		return nil
	}

	// Perform encode
	out, eerr := encoder.Encode(ctx, dv, encOpts, encoder.Options{
		FFmpegPath: ffmpegPath,
		Verbose:    parsed.Options.Verbose,
		OutputPath: outputPath,
	})
	if eerr != nil {
		return fmt.Errorf("%w: %v", errEncode, eerr)
	}

	// Write caption if requested and available
	if parsed.Options.Caption == model.CaptionTxt {
		caption := media.CaptionText(dv)
		if _, werr := util.WriteCaptionFile(out.OutputPath, caption); werr != nil {
			// Non-fatal: just warn
			fmt.Fprintf(os.Stderr, "warning: failed to write caption: %v\n", werr)
		}
	}

	// Size overshoot warning (best-effort)
	if !encOpts.ModeCRF && parsed.Options.MaxSizeMB > 0 {
		maxBytes := int64(parsed.Options.MaxSizeMB) * 1024 * 1024
		if out.Bytes > int64(float64(maxBytes)*1.10) {
			fmt.Fprintf(os.Stderr, "warning: output size (%0.2f MB) exceeds target (%d MB). Consider lowering bitrate or preset.\n",
				float64(out.Bytes)/(1024*1024), parsed.Options.MaxSizeMB)
		}
	}

	fmt.Printf("Saved: %s (%0.2f MB)\n", out.OutputPath, float64(out.Bytes)/(1024*1024))
	return nil
}

func planResolutionAndCRF(opts model.CLIOptions, dv model.DownloadedVideo, presetCRF int) (int, int) {
	target := opts.Resolution
	if target <= 0 {
		// Fallback to 720 if somehow unset
		target = 720
	}
	// Avoid upscaling if input dimensions known and smaller than target
	inLong := max(dv.Width, dv.Height)
	if inLong > 0 && inLong < target {
		target = inLong
	}
	return target, presetCRF
}

func buildOutputBasename(dv model.DownloadedVideo, longSide int, maxSizeMB int, enc model.EncodeOptions) string {
	uploader := dv.Uploader
	if uploader == "" {
		uploader = "ig"
	}
	id := dv.ID
	if id == "" {
		id = dv.Title
	}
	uploader = util.SanitizeFilename(uploader)
	id = util.SanitizeFilename(id)

	parts := []string{uploader, id}
	if enc.AudioOnly {
		parts = append(parts, "audio")
	} else {
		parts = append(parts, fmt.Sprintf("%dp", longSide))
		if enc.ModeCRF {
			parts = append(parts, fmt.Sprintf("CRF%d", enc.CRF))
		} else if maxSizeMB > 0 {
			parts = append(parts, fmt.Sprintf("%dMB", maxSizeMB))
		}
	}
	return strings.Join(parts, "_")
}

func buildCaptionText(dv model.DownloadedVideo) string {
	var b strings.Builder
	title := strings.TrimSpace(dv.Title)
	uploader := strings.TrimSpace(dv.Uploader)
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	}
	if uploader != "" {
		b.WriteString(uploader)
		b.WriteString("\n")
	}
	if dv.URL != "" {
		b.WriteString(dv.URL)
		b.WriteString("\n")
	}
	b.WriteString("\n---\nORIGINAL CAPTION\n")
	if dv.Description != "" {
		b.WriteString(dv.Description)
		b.WriteString("\n")
	}
	return b.String()
}

func findDownloader(custom string) (string, error) {
	// 1. Custom path if provided
	if custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom, nil
		}
		// Try to look up in PATH by that name
		if p, err := exec.LookPath(custom); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%w: could not find downloader at %q", errMissingDep, custom)
	}
	// 2. yt-dlp preferred
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	// 3. youtube-dl fallback
	if p, err := exec.LookPath("youtube-dl"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%w: could not find yt-dlp or youtube-dl in PATH. Please install yt-dlp and try again.", errMissingDep)
}

func findFFmpeg() (string, error) {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%w: could not find ffmpeg in PATH. Please install ffmpeg and try again.", errMissingDep)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// printPlan outputs a dry-run plan of actions without executing them.
func printPlan(rawURL, dlPath, ffmpegPath, tempDir, outputPath string, dv model.DownloadedVideo, enc model.EncodeOptions, opts model.CLIOptions) {
	fmt.Println("Dry-run plan:")
	fmt.Printf("- URL:            %s\n", rawURL)
	fmt.Printf("- Downloader:     %s\n", dlPath)
	fmt.Printf("- FFmpeg:         %s\n", ffmpegPath)
	fmt.Printf("- Temp dir:       %s\n", tempDir)
	fmt.Printf("- Output dir:     %s\n", opts.OutDir)
	fmt.Printf("- Output path:    %s\n", outputPath)
	fmt.Printf("- Audio only:     %v\n", enc.AudioOnly)
	if !enc.AudioOnly {
		fmt.Printf("- Resolution:     %dp (long side)\n", enc.LongSidePx)
		if enc.ModeCRF {
			fmt.Printf("- Mode:           CRF %d\n", enc.CRF)
		} else {
			// Compute bitrate for display
			kbps := 0
			if dv.DurationSec > 0 && opts.MaxSizeMB > 0 {
				kbps = bitrateForPreview(opts.MaxSizeMB, dv.DurationSec, enc.AudioBitrateKbps, enc.VideoMinKbps, enc.VideoMaxKbps)
			}
			fmt.Printf("- Mode:           Size-constrained (target %d MB), est video bitrate ~ %d kbps\n", opts.MaxSizeMB, kbps)
		}
	} else {
		fmt.Printf("- Audio bitrate:  %d kbps (AAC)\n", enc.AudioBitrateKbps)
	}
	fmt.Printf("- Caption:        %s\n", strings.ToUpper(string(opts.Caption)))
}

func bitrateForPreview(maxSizeMB int, durationSec float64, audioKbps, vMin, vMax int) int {
	// Mirror encoder.computeVideoBitrateKbps (cannot import unexported symbol).
	if durationSec <= 0 {
		return clamp(2000, vMin, vMax)
	}
	targetSizeBytes := int64(maxSizeMB) * 1024 * 1024
	totalBitrateBps := float64(targetSizeBytes*8) / durationSec
	videoBitrateBps := totalBitrateBps - float64(audioKbps*1000)
	kbps := int(videoBitrateBps / 1000.0)
	return clamp(kbps, vMin, vMax)
}

func clamp(v, min, max int) int {
	if min != 0 && v < min {
		return min
	}
	if max != 0 && v > max {
		return max
	}
	return v
}
