package encoder

import (
	"fmt"
	"strconv"

	"ig2wa/internal/model"
	"ig2wa/internal/util/bitrate"
)

// BuildVideoArgs constructs ffmpeg arguments for video encoding.
// Returns the arguments slice and the used CRF/bitrate values.
func BuildVideoArgs(in model.DownloadedVideo, enc model.EncodeOptions, outputPath string, includeProgress bool) (args []string, usedCRF int, usedBitrateKbps int) {
	vf, _ := scaleFilter(enc.LongSidePx, in.Width, in.Height)

	args = []string{
		"-y",
		"-i", in.InputPath,
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", valueOr(enc.Preset, "veryfast"),
		"-profile:v", valueOr(enc.Profile, "main"),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", bitrate.SafeAudioKbps(enc.AudioBitrateKbps)),
		"-movflags", "+faststart",
	}

	if enc.KeyInt > 0 {
		args = append(args, "-g", strconv.Itoa(enc.KeyInt), "-keyint_min", strconv.Itoa(enc.KeyInt))
	}

	if enc.ModeCRF {
		usedCRF = nonZero(enc.CRF, 22)
		args = append(args, "-crf", strconv.Itoa(usedCRF))
	} else {
		// bitrate mode
		kbps := bitrate.ComputeVideoKbps(enc.MaxSizeMB, in.DurationSec, bitrate.SafeAudioKbps(enc.AudioBitrateKbps), enc.VideoMinKbps, enc.VideoMaxKbps)
		usedBitrateKbps = kbps
		args = append(args, "-b:v", fmt.Sprintf("%dk", kbps))
	}

	if includeProgress {
		args = append(args, "-progress", "pipe:1", "-nostats")
	}

	args = append(args, outputPath)
	return args, usedCRF, usedBitrateKbps
}

// BuildAudioOnlyArgs constructs ffmpeg arguments for audio-only encoding.
func BuildAudioOnlyArgs(inputPath string, enc model.EncodeOptions, outputPath string, includeProgress bool) []string {
	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", nonZero(enc.AudioBitrateKbps, 128)),
		"-movflags", "+faststart",
	}

	if includeProgress {
		args = append(args, "-progress", "pipe:1", "-nostats")
	}

	args = append(args, outputPath)
	return args
}