// Package pipeline provides planning and orchestration for the sniplette workflow.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ig2wa/internal/downloader"
	"ig2wa/internal/encoder"
	"ig2wa/internal/model"
	"ig2wa/internal/progress"
	"ig2wa/internal/util"
	"ig2wa/internal/util/bitrate"
	"ig2wa/internal/util/format"
	"ig2wa/internal/util/media"
)

// Service orchestrates the download → plan → encode → finalize workflow.
type Service struct {
	dlPath     string
	ffmpegPath string
	opts       model.CLIOptions
	presetCRF  int
	runner     util.CmdRunner
	reporter   progress.Reporter
	jobID      string
}

// Option configures a Service.
type Option func(*Service)

// WithDownloaderPath sets the downloader (yt-dlp/youtube-dl) binary path.
func WithDownloaderPath(p string) Option {
	return func(s *Service) {
		s.dlPath = p
	}
}

// WithFFmpegPath sets the ffmpeg binary path.
func WithFFmpegPath(p string) Option {
	return func(s *Service) {
		s.ffmpegPath = p
	}
}

// WithCLIOptions sets the CLI options used for planning and execution.
func WithCLIOptions(o model.CLIOptions) Option {
	return func(s *Service) {
		s.opts = o
	}
}

// WithPresetCRF overrides the default CRF derived from the quality preset.
func WithPresetCRF(crf int) Option {
	return func(s *Service) {
		s.presetCRF = crf
	}
}

// WithRunner injects a custom command runner (useful for testing).
func WithRunner(r util.CmdRunner) Option {
	return func(s *Service) {
		s.runner = r
	}
}

// WithReporter attaches a progress reporter (used by TUI).
func WithReporter(rp progress.Reporter) Option {
	return func(s *Service) {
		s.reporter = rp
	}
}

// WithJobID sets the job ID associated with reporter events.
func WithJobID(id string) Option {
	return func(s *Service) {
		s.jobID = id
	}
}

// NewService constructs a new Service with the provided options.
// It applies sensible defaults for missing components.
func NewService(opts ...Option) *Service {
	s := &Service{}
	for _, o := range opts {
		o(s)
	}
	// Default runner
	if s.runner == nil {
		s.runner = util.NewDefaultRunner()
	}
	// Default CRF if not set
	if s.presetCRF == 0 {
		s.presetCRF = DefaultCRF(s.opts.Quality)
	}
	return s
}

// Plan contains the computed plan for a job (primarily for dry-run/introspection).
type Plan struct {
	OutputPath          string
	Enc                 model.EncodeOptions
	EstVideoBitrateKbps int

	DownloaderPath string
	FFmpegPath     string
	TempDir        string

	AudioOnly   bool
	LongSidePx  int
	URL         string
	Title       string
	Uploader    string
	ID          string
	DurationSec float64
}

// Result returns the outcome of RunJob.
type Result struct {
	URL            string
	Planned        bool
	Plan           *Plan
	Output         *model.OutputVideo
	DV             model.DownloadedVideo
	Overshot       bool
	OvershootRatio float64
	TempDir        string
}

// RunJob executes the full pipeline for a single URL.
// It never prints; when a Reporter is present, it emits progress and a final Result.
func (s *Service) RunJob(ctx context.Context, url string) (Result, error) {
	var res Result
	res.URL = url

	// Basic validation: downloader is always required (even for dry-run metadata).
	if s.dlPath == "" {
		return res, fmt.Errorf("downloader path is required")
	}
	// ffmpeg required only when not dry-run (encoding path).
	if !s.opts.DryRun && s.ffmpegPath == "" {
		return res, fmt.Errorf("ffmpeg path is required")
	}

	// Step 1: Download metadata (or full media if not dry-run)
	dlOpts := s.makeDownloaderOptions(s.opts.DryRun, s.jobID)
	dv, tempDir, derr := downloader.Download(ctx, url, dlOpts)
	// Ensure cleanup unless KeepTemp is set
	defer func() {
		if !s.opts.KeepTemp && tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()
	// Preserve tempDir for diagnostics or KeepTemp
	if s.opts.KeepTemp {
		res.TempDir = tempDir
	}

	if derr != nil {
		// Return partial result with error for diagnostics
		res.DV = dv
		if res.TempDir == "" {
			res.TempDir = tempDir
		}
		return res, fmt.Errorf("downloader: %w", derr)
	}
	res.DV = dv

	// Step 2: Plan (resolution, CRF/bitrate, output path)
	enc, outputPath, estVideoKbps := s.plan(dv)

	// Dry-run path
	if s.opts.DryRun {
		pl := &Plan{
			OutputPath:          outputPath,
			Enc:                 enc,
			EstVideoBitrateKbps: estVideoKbps,
			DownloaderPath:      s.dlPath,
			FFmpegPath:          s.ffmpegPath,
			TempDir:             "", // report only when keep-temp requested
			AudioOnly:           enc.AudioOnly,
			LongSidePx:          enc.LongSidePx,
			URL:                 url,
			Title:               dv.Title,
			Uploader:            dv.Uploader,
			ID:                  dv.ID,
			DurationSec:         dv.DurationSec,
		}
		if s.opts.KeepTemp {
			pl.TempDir = tempDir
		}
		res.Planned = true
		res.Plan = pl

		// TUI progress: emit planned and final result for UI consumption
		s.emitPlanned(outputPath)
		return res, nil
	}

	// Step 3: Encode
	out, eerr := encoder.Encode(ctx, dv, enc, encoder.Options{
		FFmpegPath: s.ffmpegPath,
		Verbose:    s.opts.Verbose,
		OutputPath: outputPath,
		Reporter:   s.reporter,
		JobID:      s.jobID,
		Runner:     s.runner,
	})
	if eerr != nil {
		if res.TempDir == "" {
			res.TempDir = tempDir
		}
		return res, fmt.Errorf("encode: %w", eerr)
	}

	// Step 4: Finalize (caption, reporting, overshoot)
	if s.opts.Caption == model.CaptionTxt {
		s.writeCaptionIfNeeded(dv, out.OutputPath)
	}

	s.emitSaved(out)

	overshot, ratio := s.checkOvershoot(out.Bytes)
	res.Output = &out
	res.Overshot = overshot
	res.OvershootRatio = ratio

	return res, nil
}

// makeDownloaderOptions constructs downloader.Options with injected dependencies.
func (s *Service) makeDownloaderOptions(metaOnly bool, jobID string) downloader.Options {
	return downloader.Options{
		DownloaderPath: s.dlPath,
		Verbose:        s.opts.Verbose,
		KeepTemp:       s.opts.KeepTemp,
		MetadataOnly:   metaOnly,
		Reporter:       s.reporter,
		JobID:          jobID,
		Runner:         s.runner,
	}
}

// plan computes encoding options, output path, and estimated bitrate (when applicable).
func (s *Service) plan(dv model.DownloadedVideo) (model.EncodeOptions, string, int) {
	targetLongSide, usedCRF := PlanResolutionAndCRF(s.opts, dv, s.presetCRF)
	enc := model.EncodeOptions{
		LongSidePx:       targetLongSide,
		ModeCRF:          s.opts.MaxSizeMB == 0 || dv.DurationSec <= 0 || s.opts.AudioOnly,
		CRF:              usedCRF,
		MaxSizeMB:        s.opts.MaxSizeMB,
		AudioBitrateKbps: 96,
		VideoMinKbps:     500,
		VideoMaxKbps:     8000,
		Preset:           "veryfast",
		Profile:          "main",
		AudioOnly:        s.opts.AudioOnly,
		KeyInt:           48,
	}

	base := media.OutputBasename(dv, targetLongSide, s.opts.MaxSizeMB, enc)
	ext := ".mp4"
	if s.opts.AudioOnly {
		ext = ".m4a"
	}
	outPath := filepath.Join(s.opts.OutDir, base+ext)

	estKbps := 0
	if !enc.ModeCRF && s.opts.MaxSizeMB > 0 && dv.DurationSec > 0 && !enc.AudioOnly {
		estKbps = bitrate.ComputeVideoKbps(
			s.opts.MaxSizeMB,
			dv.DurationSec,
			bitrate.SafeAudioKbps(enc.AudioBitrateKbps),
			enc.VideoMinKbps,
			enc.VideoMaxKbps,
		)
	}

	return enc, outPath, estKbps
}

// emitPlanned sends a final "planned" update and reporter result for TUI.
func (s *Service) emitPlanned(outPath string) {
	if s.reporter == nil {
		return
	}
	name := filepath.Base(outPath)
	s.reporter.Update(progress.Update{
		JobID:   s.jobID,
		Stage:   progress.StageCompleted,
		Percent: 100,
		Message: fmt.Sprintf("Planned: %s (dry-run)", name),
	})
	s.reporter.Result(progress.Result{
		JobID:      s.jobID,
		OutputPath: outPath,
		Bytes:      0,
		Err:        nil,
	})
}

// emitSaved sends a final "saved" update and reporter result for TUI.
func (s *Service) emitSaved(out model.OutputVideo) {
	if s.reporter == nil {
		return
	}
	name := filepath.Base(out.OutputPath)
	size := format.HumanizeBytes(out.Bytes)
	s.reporter.Update(progress.Update{
		JobID:   s.jobID,
		Stage:   progress.StageCompleted,
		Percent: 100,
		Message: fmt.Sprintf("Saved: %s (%s)", name, size),
	})
	s.reporter.Result(progress.Result{
		JobID:      s.jobID,
		OutputPath: out.OutputPath,
		Bytes:      out.Bytes,
		Err:        nil,
	})
}

// checkOvershoot determines whether the output size exceeds the max target by >10%.
func (s *Service) checkOvershoot(outBytes int64) (bool, float64) {
	if s.opts.MaxSizeMB <= 0 {
		return false, 0
	}
	maxBytes := int64(s.opts.MaxSizeMB) * 1024 * 1024
	if maxBytes <= 0 {
		return false, 0
	}
	ratio := float64(outBytes) / float64(maxBytes)
	return ratio > 1.10, ratio
}

// writeCaptionIfNeeded writes a sidecar caption file if enabled.
// Logs a warning via reporter in verbose mode on failure (best-effort).
func (s *Service) writeCaptionIfNeeded(dv model.DownloadedVideo, outputPath string) {
	if s.opts.Caption != model.CaptionTxt {
		return
	}
	caption := media.CaptionText(dv)
	if _, err := util.WriteCaptionFile(outputPath, caption); err != nil && s.reporter != nil && s.opts.Verbose {
		s.reporter.Log(progress.Log{
			JobID:  s.jobID,
			Stream: progress.StreamStderr,
			Line:   fmt.Sprintf("warning: failed to write caption: %v", err),
		})
	}
}