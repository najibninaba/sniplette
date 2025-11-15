package encoder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"ig2wa/internal/model"
	"ig2wa/internal/util"
)

// Options control ffmpeg execution.
type Options struct {
	FFmpegPath string
	Verbose    bool
	OutputPath string // Full path of desired output file (including extension)
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

	vf, _ := scaleFilter(enc.LongSidePx, in.Width, in.Height)
	args := []string{
		"-y",
		"-i", in.InputPath,
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", valueOr(enc.Preset, "veryfast"),
		"-profile:v", valueOr(enc.Profile, "main"),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", safeAudioKbps(enc.AudioBitrateKbps)),
		"-movflags", "+faststart",
	}
	if enc.KeyInt > 0 {
		args = append(args, "-g", strconv.Itoa(enc.KeyInt), "-keyint_min", strconv.Itoa(enc.KeyInt))
	}

	usedCRF := 0
	usedVBR := 0
	if enc.ModeCRF {
		usedCRF = nonZero(enc.CRF, 22)
		args = append(args, "-crf", strconv.Itoa(usedCRF))
	} else {
		// bitrate mode
		if in.DurationSec <= 0 || enc.MaxSizeMB <= 0 {
			return model.OutputVideo{}, errors.New("invalid bitrate mode inputs: missing duration or max size")
		}
		kbps := computeVideoBitrateKbps(enc.MaxSizeMB, in.DurationSec, safeAudioKbps(enc.AudioBitrateKbps), enc.VideoMinKbps, enc.VideoMaxKbps)
		usedVBR = kbps
		args = append(args, "-b:v", fmt.Sprintf("%dk", kbps))
	}

	if opts.OutputPath == "" {
		return model.OutputVideo{}, errors.New("output path is required")
	}
	args = append(args, opts.OutputPath)

	// Ensure output dir exists
	if err := util.EnsureDir(filepath.Dir(opts.OutputPath)); err != nil {
		return model.OutputVideo{}, fmt.Errorf("ensure output dir: %w", err)
	}

	_, runErr := util.Run(ctx, util.CmdSpec{
		Path:    opts.FFmpegPath,
		Args:    args,
		Verbose: opts.Verbose,
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
	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", nonZero(enc.AudioBitrateKbps, 128)),
		"-movflags", "+faststart",
		opts.OutputPath,
	}
	if err := util.EnsureDir(filepath.Dir(opts.OutputPath)); err != nil {
		return model.OutputVideo{}, fmt.Errorf("ensure output dir: %w", err)
	}
	_, runErr := util.Run(ctx, util.CmdSpec{
		Path:    opts.FFmpegPath,
		Args:    args,
		Verbose: opts.Verbose,
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
