package encoder

import (
	"strings"
	"testing"

	"ig2wa/internal/model"
)

func TestBuildVideoArgs(t *testing.T) {
	tests := []struct {
		name            string
		in              model.DownloadedVideo
		enc             model.EncodeOptions
		outputPath      string
		includeProgress bool
		wantCRF         int
		wantBitrate     int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "CRF mode",
			in: model.DownloadedVideo{
				InputPath:   "/tmp/input.mp4",
				DurationSec: 60.0,
				Width:       1920,
				Height:      1080,
			},
			enc: model.EncodeOptions{
				ModeCRF:          true,
				CRF:              23,
				LongSidePx:       1920,
				AudioBitrateKbps: 128,
				Preset:           "fast",
				Profile:          "high",
			},
			outputPath:      "/tmp/output.mp4",
			includeProgress: false,
			wantCRF:         23,
			wantBitrate:     0,
			wantContains:    []string{"-crf", "23", "-preset", "fast", "-profile:v", "high", "-b:a", "128k"},
			wantNotContains: []string{"-b:v", "-progress"},
		},
		{
			name: "bitrate mode",
			in: model.DownloadedVideo{
				InputPath:   "/tmp/input.mp4",
				DurationSec: 60.0,
				Width:       1280,
				Height:      720,
			},
			enc: model.EncodeOptions{
				ModeCRF:          false,
				MaxSizeMB:        50,
				LongSidePx:       720,
				AudioBitrateKbps: 128,
				VideoMinKbps:     500,
				VideoMaxKbps:     5000,
			},
			outputPath:      "/tmp/output.mp4",
			includeProgress: true,
			wantCRF:         0,
			wantBitrate:     5000, // Will be clamped to max
			wantContains:    []string{"-b:v", "-progress", "pipe:1", "-nostats"},
			wantNotContains: []string{"-crf"},
		},
		{
			name: "with keyframe interval",
			in: model.DownloadedVideo{
				InputPath:   "/tmp/input.mp4",
				DurationSec: 60.0,
				Width:       1280,
				Height:      720,
			},
			enc: model.EncodeOptions{
				ModeCRF:          true,
				CRF:              22,
				LongSidePx:       720,
				AudioBitrateKbps: 128,
				KeyInt:           60,
			},
			outputPath:      "/tmp/output.mp4",
			includeProgress: false,
			wantCRF:         22,
			wantContains:    []string{"-g", "60", "-keyint_min", "60"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, gotCRF, gotBitrate := BuildVideoArgs(tt.in, tt.enc, tt.outputPath, tt.includeProgress)

			if gotCRF != tt.wantCRF {
				t.Errorf("BuildVideoArgs() CRF = %v, want %v", gotCRF, tt.wantCRF)
			}
			if gotBitrate != tt.wantBitrate {
				t.Errorf("BuildVideoArgs() bitrate = %v, want %v", gotBitrate, tt.wantBitrate)
			}

			argsStr := strings.Join(args, " ")
			for _, want := range tt.wantContains {
				if !strings.Contains(argsStr, want) {
					t.Errorf("BuildVideoArgs() args missing %q, got: %v", want, args)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(argsStr, notWant) {
					t.Errorf("BuildVideoArgs() args should not contain %q, got: %v", notWant, args)
				}
			}

			// Verify output path is last arg
			if args[len(args)-1] != tt.outputPath {
				t.Errorf("BuildVideoArgs() last arg = %v, want %v", args[len(args)-1], tt.outputPath)
			}
		})
	}
}

func TestBuildAudioOnlyArgs(t *testing.T) {
	tests := []struct {
		name            string
		inputPath       string
		enc             model.EncodeOptions
		outputPath      string
		includeProgress bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:       "basic audio only",
			inputPath:  "/tmp/input.mp4",
			enc:        model.EncodeOptions{AudioBitrateKbps: 128},
			outputPath: "/tmp/output.m4a",
			wantContains: []string{
				"-vn",
				"-c:a", "aac",
				"-b:a", "128k",
				"-movflags", "+faststart",
			},
			wantNotContains: []string{"-vf", "-c:v"},
		},
		{
			name:            "with progress",
			inputPath:       "/tmp/input.mp4",
			enc:             model.EncodeOptions{AudioBitrateKbps: 192},
			outputPath:      "/tmp/output.m4a",
			includeProgress: true,
			wantContains:    []string{"-progress", "pipe:1", "-nostats"},
		},
		{
			name:       "default bitrate when zero",
			inputPath:  "/tmp/input.mp4",
			enc:        model.EncodeOptions{AudioBitrateKbps: 0},
			outputPath: "/tmp/output.m4a",
			wantContains: []string{
				"-b:a", "128k", // defaults to 128
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildAudioOnlyArgs(tt.inputPath, tt.enc, tt.outputPath, tt.includeProgress)

			argsStr := strings.Join(args, " ")
			for _, want := range tt.wantContains {
				if !strings.Contains(argsStr, want) {
					t.Errorf("BuildAudioOnlyArgs() args missing %q, got: %v", want, args)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(argsStr, notWant) {
					t.Errorf("BuildAudioOnlyArgs() args should not contain %q, got: %v", notWant, args)
				}
			}

			if args[len(args)-1] != tt.outputPath {
				t.Errorf("BuildAudioOnlyArgs() last arg = %v, want %v", args[len(args)-1], tt.outputPath)
			}
		})
	}
}