package encoder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"ig2wa/internal/model"
	"ig2wa/internal/progress"
	"ig2wa/internal/util"
)

// Options control ffmpeg execution.
type Options struct {
	FFmpegPath string
	Verbose    bool
	OutputPath string // Full path of desired output file (including extension)

	// Progress reporting (optional)
	Reporter progress.Reporter
	JobID    string

	// Runner is optional; if nil, defaults to util.NewDefaultRunner()
	Runner util.CmdRunner
}

// Encode performs the transcoding according to the provided options.
// It returns metadata about the resulting file on success.
func Encode(ctx context.Context, in model.DownloadedVideo, enc model.EncodeOptions, opts Options) (model.OutputVideo, error) {
	if opts.FFmpegPath == "" {
		return model.OutputVideo{}, errors.New("ffmpeg path is required")
	}
	if enc.AudioOnly {
		if opts.OutputPath == "" {
			return model.OutputVideo{}, errors.New("output path is required")
		}
		return encodeAudioOnly(ctx, in.InputPath, opts, enc)
	}

	// Initialize runner with default if nil
	runner := opts.Runner
	if runner == nil {
		runner = util.NewDefaultRunner()
	}

	includeProgress := opts.Reporter != nil && !opts.Verbose
	args, usedCRF, usedVBR := BuildVideoArgs(in, enc, opts.OutputPath, includeProgress)

	if opts.OutputPath == "" {
		return model.OutputVideo{}, errors.New("output path is required")
	}

	// Ensure output dir exists
	if err := util.EnsureDir(filepath.Dir(opts.OutputPath)); err != nil {
		return model.OutputVideo{}, fmt.Errorf("ensure output dir: %w", err)
	}

	if opts.Reporter != nil {
		opts.Reporter.Update(progress.Update{
			JobID:   opts.JobID,
			Stage:   progress.StageEncoding,
			Percent: 0,
			Message: "Encoding",
		})
	}

	// Track progress state
	var progState ProgressState

	_, runErr := runner.Run(ctx, util.CmdSpec{
		Path:          opts.FFmpegPath,
		Args:          args,
		Verbose:       opts.Verbose && opts.Reporter == nil,
		CaptureStdout: opts.Reporter == nil,
		StdoutLine: func(line string) {
			if opts.Reporter == nil {
				return
			}
			if u, ok := progState.UpdateFromLine(line, opts.JobID, in.DurationSec, false); ok {
				opts.Reporter.Update(u)
			}
			if opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStdout, Line: line})
			}
		},
		StderrLine: func(line string) {
			if opts.Reporter != nil && opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStderr, Line: line})
			}
		},
	})
	if runErr != nil {
		// Delete incomplete file
		_ = util.RemoveIfExists(opts.OutputPath)
		return model.OutputVideo{}, fmt.Errorf("ffmpeg failed: %w", runErr)
	}

	// Stat output
	fi, err := os.Stat(opts.OutputPath)
	if err != nil {
		return model.OutputVideo{}, fmt.Errorf("stat output: %w", err)
	}

	return model.OutputVideo{
		OutputPath:      opts.OutputPath,
		Bytes:           fi.Size(),
		UsedCRF:         usedCRF,
		UsedBitrateKbps: usedVBR,
		LongSidePx:      enc.LongSidePx,
		AudioOnly:       false,
	}, nil
}

// computeVideoBitrateKbps calculates a video bitrate to fit within a target size.
func computeVideoBitrateKbps(maxSizeMB int, durationSec float64, audioKbps, vMinKbps, vMaxKbps int) int {
	if durationSec <= 0 {
		return clamp(2000, vMinKbps, vMaxKbps)
	}
	targetSizeBytes := int64(maxSizeMB) * 1024 * 1024
	totalBitrateBps := float64(targetSizeBytes*8) / durationSec
	videoBitrateBps := totalBitrateBps - float64(audioKbps*1000)
	kbps := int(videoBitrateBps / 1000.0)
	if kbps < vMinKbps {
		kbps = vMinKbps
	}
	if kbps > vMaxKbps {
		kbps = vMaxKbps
	}
	return kbps
}

// scaleFilter returns the ffmpeg scale filter and whether the input is vertical.
func scaleFilter(longSide int, width, height int) (string, bool) {
	if longSide <= 0 {
		longSide = 720
	}
	vertical := height > width && height > 0 && width > 0
	if vertical {
		return fmt.Sprintf("scale=-2:%d", longSide), true
	}
	return fmt.Sprintf("scale=%d:-2", longSide), false
}

func encodeAudioOnly(ctx context.Context, inputPath string, opts Options, enc model.EncodeOptions) (model.OutputVideo, error) {
	if inputPath == "" {
		return model.OutputVideo{}, errors.New("input path is required")
	}
	// Initialize runner with default if nil
	runner := opts.Runner
	if runner == nil {
		runner = util.NewDefaultRunner()
	}

	includeProgress := opts.Reporter != nil && !opts.Verbose
	args := BuildAudioOnlyArgs(inputPath, enc, opts.OutputPath, includeProgress)

	if err := util.EnsureDir(filepath.Dir(opts.OutputPath)); err != nil {
		return model.OutputVideo{}, fmt.Errorf("ensure output dir: %w", err)
	}

	if opts.Reporter != nil {
		opts.Reporter.Update(progress.Update{
			JobID:   opts.JobID,
			Stage:   progress.StageEncoding,
			Percent: 0,
			Message: "Encoding (audio)",
		})
	}

	var progState ProgressState

	_, runErr := runner.Run(ctx, util.CmdSpec{
		Path:          opts.FFmpegPath,
		Args:          args,
		Verbose:       opts.Verbose && opts.Reporter == nil,
		CaptureStdout: opts.Reporter == nil,
		StdoutLine: func(line string) {
			if opts.Reporter == nil {
				return
			}
			if u, ok := progState.UpdateFromLine(line, opts.JobID, 0, true); ok {
				opts.Reporter.Update(u)
			}
			if opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStdout, Line: line})
			}
		},
		StderrLine: func(line string) {
			if opts.Reporter != nil && opts.Verbose {
				opts.Reporter.Log(progress.Log{JobID: opts.JobID, Stream: progress.StreamStderr, Line: line})
			}
		},
	})
	if runErr != nil {
		_ = util.RemoveIfExists(opts.OutputPath)
		return model.OutputVideo{}, fmt.Errorf("ffmpeg failed: %w", runErr)
	}
	fi, err := os.Stat(opts.OutputPath)
	if err != nil {
		return model.OutputVideo{}, fmt.Errorf("stat output: %w", err)
	}
	return model.OutputVideo{
		OutputPath:      opts.OutputPath,
		Bytes:           fi.Size(),
		UsedCRF:         0,
		UsedBitrateKbps: 0,
		LongSidePx:      0,
		AudioOnly:       true,
	}, nil
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

func valueOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nonZero(v int, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func safeAudioKbps(v int) int {
	if v <= 0 {
		return 96
	}
	if v < 32 {
		return 32
	}
	if v > 320 {
		return 320
	}
	return v
}
