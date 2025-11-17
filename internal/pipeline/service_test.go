package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ig2wa/internal/model"
	"ig2wa/internal/progress"
	"ig2wa/internal/util"
)

type recordingReporter struct {
	updates []progress.Update
	results []progress.Result
	logs    []progress.Log
}

func (r *recordingReporter) Update(u progress.Update) {
	r.updates = append(r.updates, u)
}
func (r *recordingReporter) Log(l progress.Log) {
	r.logs = append(r.logs, l)
}
func (r *recordingReporter) Result(res progress.Result) {
	r.results = append(r.results, res)
}

type fakeRunner struct {
	t                *testing.T
	dlPath           string
	ffmpegPath       string
	metaJSON         string
	videoID          string
	downloadedExt    string
	ffmpegOutputSize int64
}

// Run implements util.CmdRunner.Run and simulates yt-dlp and ffmpeg behavior.
func (f *fakeRunner) Run(ctx context.Context, spec util.CmdSpec) (util.CmdResult, error) {
	// Simulate downloader metadata fetch
	if spec.Path == f.dlPath {
		// Metadata path (yt-dlp --dump-json)
		if contains(spec.Args, "--dump-json") {
			if spec.StdoutLine != nil {
				// Emit single-line JSON if a line handler is present
				spec.StdoutLine(strings.TrimSpace(f.metaJSON))
			}
			return util.CmdResult{
				Stdout: []byte(f.metaJSON),
				Stderr: nil,
				Code:   0,
				Err:    nil,
			}, nil
		}

		// Simulate the actual download step by creating the expected file in workdir
		workdir := spec.Dir
		if workdir == "" {
			f.t.Fatalf("downloader run missing working dir")
		}
		ext := f.downloadedExt
		if ext == "" {
			ext = ".mp4"
		}
		out := filepath.Join(workdir, f.videoID+ext)
		if err := os.WriteFile(out, []byte("downloaded"), 0o644); err != nil {
			f.t.Fatalf("failed to create fake downloaded file: %v", err)
		}
		// Emit progress-like lines if requested
		if spec.StdoutLine != nil {
			spec.StdoutLine("[download]  50.0% of 10.00MiB at  1.0MiB/s ETA 00:04")
			spec.StdoutLine("[download] 100.0% of 10.00MiB at  1.0MiB/s ETA 00:00")
		}
		return util.CmdResult{
			Stdout: nil,
			Stderr: nil,
			Code:   0,
			Err:    nil,
		}, nil
	}

	// Simulate ffmpeg run
	if spec.Path == f.ffmpegPath {
		if len(spec.Args) == 0 {
			return util.CmdResult{}, errors.New("no args")
		}
		outputPath := spec.Args[len(spec.Args)-1]
		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return util.CmdResult{}, err
		}
		// Create output file with the desired size
		size := f.ffmpegOutputSize
		if size <= 0 {
			size = 1024 // default 1KB
		}
		data := make([]byte, size)
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			return util.CmdResult{}, err
		}
		// Emit ffmpeg progress lines if requested
		if spec.StdoutLine != nil {
			spec.StdoutLine("out_time_ms=1000000")
			spec.StdoutLine("speed=1.0x")
			spec.StdoutLine("total_size=1024")
			spec.StdoutLine("progress=continue")
			spec.StdoutLine("out_time_ms=2000000")
			spec.StdoutLine("progress=end")
		}
		return util.CmdResult{
			Stdout: nil,
			Stderr: nil,
			Code:   0,
			Err:    nil,
		}, nil
	}

	// Unknown tool path
	return util.CmdResult{}, errors.New("unexpected tool path: " + spec.Path)
}

// contains helper
func contains(ss []string, q string) bool {
	for _, s := range ss {
		if s == q {
			return true
		}
	}
	return false
}

// ---------- Tests ----------

func TestNewService_WithOptions(t *testing.T) {
	opts := model.CLIOptions{
		OutDir:     "out",
		MaxSizeMB:  50,
		Quality:    model.PresetHigh,
		Resolution: 1080,
		AudioOnly:  false,
		Caption:    model.CaptionTxt,
		KeepTemp:   true,
		DLBinary:   "yt-dlp",
		DryRun:     false,
		Verbose:    true,
		NoUI:       true,
		Jobs:       1,
	}
	r := &fakeRunner{}
	rep := &recordingReporter{}

	s := NewService(
		WithDownloaderPath("/usr/local/bin/yt-dlp"),
		WithFFmpegPath("/usr/local/bin/ffmpeg"),
		WithCLIOptions(opts),
		WithPresetCRF(21),
		WithRunner(r),
		WithReporter(rep),
		WithJobID("job-1"),
	)

	if s.dlPath != "/usr/local/bin/yt-dlp" {
		t.Errorf("dlPath = %q", s.dlPath)
	}
	if s.ffmpegPath != "/usr/local/bin/ffmpeg" {
		t.Errorf("ffmpegPath = %q", s.ffmpegPath)
	}
	if s.opts.OutDir != "out" || s.opts.MaxSizeMB != 50 || s.opts.Quality != model.PresetHigh {
		t.Errorf("opts not set correctly: %+v", s.opts)
	}
	if s.presetCRF != 21 {
		t.Errorf("presetCRF = %d, want 21", s.presetCRF)
	}
	if s.runner == nil {
		t.Error("runner not set")
	}
	if s.reporter == nil {
		t.Error("reporter not set")
	}
	if s.jobID != "job-1" {
		t.Errorf("jobID = %q", s.jobID)
	}

	// Default CRF when not provided
	s2 := NewService(WithCLIOptions(model.CLIOptions{Quality: model.PresetLow}))
	if s2.presetCRF != DefaultCRF(model.PresetLow) {
		t.Errorf("default CRF = %d, want %d", s2.presetCRF, DefaultCRF(model.PresetLow))
	}
}

func TestMakeDownloaderOptions(t *testing.T) {
	r := &fakeRunner{}
	rep := &recordingReporter{}
	s := NewService(
		WithCLIOptions(model.CLIOptions{Verbose: true, KeepTemp: true, DryRun: true}),
		WithDownloaderPath("/bin/yt-dlp"),
		WithRunner(r),
		WithReporter(rep),
		WithJobID("job-xyz"),
	)
	opts := s.makeDownloaderOptions(true, "job-xyz")
	if opts.DownloaderPath != "/bin/yt-dlp" {
		t.Errorf("DownloaderPath = %q", opts.DownloaderPath)
	}
	if !opts.Verbose || !opts.KeepTemp || !opts.MetadataOnly {
		t.Errorf("opts flags not set correctly: %+v", opts)
	}
	if opts.Reporter != rep {
		t.Errorf("Reporter mismatch")
	}
	if opts.JobID != "job-xyz" {
		t.Errorf("JobID = %q", opts.JobID)
	}
	if opts.Runner == nil {
		t.Errorf("Runner should be set")
	}
}

func TestCheckOvershoot(t *testing.T) {
	s := NewService(WithCLIOptions(model.CLIOptions{MaxSizeMB: 50}))
	// Below threshold
	o, r := s.checkOvershoot(54 * 1024 * 1024)
	if o {
		t.Errorf("expected no overshoot, got true (ratio=%.2f)", r)
	}
	// Exactly at 10% over (55MB) should be false (strict >1.10)
	o, r = s.checkOvershoot(55 * 1024 * 1024)
	if o {
		t.Errorf("expected no overshoot at exact 10%%, got true (ratio=%.2f)", r)
	}
	// Above threshold
	o, r = s.checkOvershoot(56 * 1024 * 1024)
	if !o {
		t.Errorf("expected overshoot, got false (ratio=%.2f)", r)
	}
}

func TestPlan_GeneratesExpected(t *testing.T) {
	tmp := t.TempDir()
	opts := model.CLIOptions{
		OutDir:     tmp,
		MaxSizeMB:  50,
		Quality:    model.PresetMedium,
		Resolution: 1080,
		AudioOnly:  false,
	}
	s := NewService(WithCLIOptions(opts), WithPresetCRF(22))
	dv := model.DownloadedVideo{
		Title:       "Sample Video",
		Uploader:    "Uploader",
		ID:          "vid123",
		DurationSec: 60.0,
		Width:       720,
		Height:      720,
	}
	enc, out, est := s.plan(dv)

	// No upscaling: long side should be 720 (input), not 1080
	if enc.LongSidePx != 720 {
		t.Errorf("LongSidePx = %d, want 720", enc.LongSidePx)
	}
	// Since MaxSizeMB > 0 and duration known, expect bitrate mode (ModeCRF=false)
	if enc.ModeCRF {
		t.Errorf("ModeCRF = true, want false (size-constrained)")
	}
	// Ext and OutDir
	if filepath.Dir(out) != tmp {
		t.Errorf("output dir = %q, want %q", filepath.Dir(out), tmp)
	}
	if !strings.HasSuffix(out, ".mp4") {
		t.Errorf("output path should end with .mp4, got %q", out)
	}
	// Estimated kbps present
	if est <= 0 {
		t.Errorf("estVideoKbps = %d, want >0", est)
	}

	// Audio-only path
	sAO := NewService(WithCLIOptions(model.CLIOptions{OutDir: tmp, AudioOnly: true, Resolution: 1080}))
	dvAO := dv
	enc2, out2, est2 := sAO.plan(dvAO)
	if !enc2.AudioOnly {
		t.Errorf("AudioOnly not propagated")
	}
	if !strings.HasSuffix(out2, ".m4a") {
		t.Errorf("audio-only output should end with .m4a, got %q", out2)
	}
	if est2 != 0 {
		t.Errorf("estVideoKbps for audio-only should be 0, got %d", est2)
	}
}

func TestRunJob_DryRun_Reporter(t *testing.T) {
	tmp := t.TempDir()
	dlPath := "/bin/yt-dlp"
	rep := &recordingReporter{}
	fr := &fakeRunner{
		t:             t,
		dlPath:        dlPath,
		ffmpegPath:    "/bin/ffmpeg",
		videoID:       "abc123",
		metaJSON:      `{"id":"abc123","title":"Title","uploader":"Up","duration":42,"width":1280,"height":720,"description":"desc"}`,
		downloadedExt: ".mp4",
	}

	s := NewService(
		WithDownloaderPath(dlPath),
		WithCLIOptions(model.CLIOptions{
			OutDir:  tmp,
			DryRun:  true,
			Verbose: false,
		}),
		WithRunner(fr),
		WithReporter(rep),
		WithJobID("job-1"),
		// preset not provided: will default via DefaultCRF
	)

	res, err := s.RunJob(context.Background(), "https://example.com/video")
	if err != nil {
		t.Fatalf("RunJob (dry-run) error: %v", err)
	}
	if !res.Planned || res.Plan == nil {
		t.Fatalf("expected Planned with non-nil Plan")
	}
	// Reporter should have StageCompleted update with Planned message
	if len(rep.updates) == 0 {
		t.Fatalf("expected reporter updates, got none")
	}
	last := rep.updates[len(rep.updates)-1]
	if last.Stage != progress.StageCompleted || !strings.Contains(last.Message, "Planned:") {
		t.Errorf("final update = %+v, want StageCompleted with Planned", last)
	}
	if len(rep.results) == 0 || rep.results[len(rep.results)-1].Err != nil {
		t.Errorf("expected success result, got %+v", rep.results)
	}
}

func TestRunJob_MissingPaths(t *testing.T) {
	// Missing downloader path
	s1 := NewService(WithCLIOptions(model.CLIOptions{DryRun: true}))
	_, err := s1.RunJob(context.Background(), "https://x")
	if err == nil || !strings.Contains(err.Error(), "downloader path is required") {
		t.Errorf("expected downloader path error, got %v", err)
	}

	// Missing ffmpeg path in non-dry-run
	s2 := NewService(
		WithCLIOptions(model.CLIOptions{DryRun: false}),
		WithDownloaderPath("/bin/yt-dlp"),
	)
	_, err = s2.RunJob(context.Background(), "https://x")
	if err == nil || !strings.Contains(err.Error(), "ffmpeg path is required") {
		t.Errorf("expected ffmpeg path error, got %v", err)
	}
}

func TestRunJob_EncodeAndReporter(t *testing.T) {
	tmp := t.TempDir()
	dlPath := "/bin/yt-dlp"
	ffPath := "/bin/ffmpeg"

	rep := &recordingReporter{}
	fr := &fakeRunner{
		t:                t,
		dlPath:           dlPath,
		ffmpegPath:       ffPath,
		videoID:          "xyz789",
		metaJSON:         `{"id":"xyz789","title":"Title2","uploader":"Up2","duration":60,"width":1920,"height":1080,"description":"desc"}`,
		downloadedExt:    ".mp4",
		ffmpegOutputSize: 30 * 1024 * 1024, // 30MB
	}

	s := NewService(
		WithDownloaderPath(dlPath),
		WithFFmpegPath(ffPath),
		WithCLIOptions(model.CLIOptions{
			OutDir:     tmp,
			MaxSizeMB:  50,
			DryRun:     false,
			Verbose:    false,
			Resolution: 1080,
		}),
		WithRunner(fr),
		WithReporter(rep),
		WithJobID("job-2"),
		WithPresetCRF(22),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := s.RunJob(ctx, "https://example.com/video2")
	if err != nil {
		t.Fatalf("RunJob encode error: %v", err)
	}
	if res.Output == nil {
		t.Fatalf("expected Output on success")
	}
	if res.Overshot {
		t.Errorf("unexpected overshot (bytes=%d)", res.Output.Bytes)
	}
	// Reporter should have final saved message
	if len(rep.updates) == 0 {
		t.Fatalf("expected reporter updates")
	}
	lastU := rep.updates[len(rep.updates)-1]
	if lastU.Stage != progress.StageCompleted || !strings.Contains(lastU.Message, "Saved:") {
		t.Errorf("final update = %+v, want StageCompleted with Saved", lastU)
	}
	if len(rep.results) == 0 || rep.results[len(rep.results)-1].Err != nil {
		t.Errorf("expected success result, got %+v", rep.results)
	}
}