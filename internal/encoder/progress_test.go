package encoder

import (
	"testing"

	"ig2wa/internal/progress"
)

func TestProgressState_UpdateFromLine(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string // Multiple lines to process in sequence
		jobID       string
		durationSec float64
		isAudioOnly bool
		wantOk      bool
		wantPercent float64
		wantStage   progress.Stage
	}{
		{
			name: "video progress sequence",
			lines: []string{
				"out_time_ms=30000000", // 30 seconds
				"speed=1.5x",
				"total_size=10485760",
				"progress=continue",
			},
			jobID:       "job1",
			durationSec: 60.0,
			wantOk:      true,
			wantPercent: 50.0, // 30s / 60s
			wantStage:   progress.StageEncoding,
		},
		{
			name: "audio only progress",
			lines: []string{
				"speed=2.0x",
				"total_size=5242880",
				"progress=continue",
			},
			jobID:       "job2",
			durationSec: 0,
			isAudioOnly: true,
			wantOk:      true,
			wantPercent: -1.0, // Unknown for audio-only
			wantStage:   progress.StageEncoding,
		},
		{
			name: "completion progress",
			lines: []string{
				"out_time_ms=60000000", // Full duration
				"progress=end",
			},
			jobID:       "job3",
			durationSec: 60.0,
			wantOk:      true,
			wantPercent: 100.0,
			wantStage:   progress.StageEncoding,
		},
		{
			name:        "non-progress line",
			lines:       []string{"frame=100"},
			jobID:       "job4",
			durationSec: 60.0,
			wantOk:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &ProgressState{}
			var u progress.Update
			var ok bool

			// Process all lines
			for _, line := range tt.lines {
				u, ok = ps.UpdateFromLine(line, tt.jobID, tt.durationSec, tt.isAudioOnly)
			}

			if ok != tt.wantOk {
				t.Errorf("UpdateFromLine() ok = %v, want %v", ok, tt.wantOk)
			}

			if !tt.wantOk {
				return
			}

			if u.JobID != tt.jobID {
				t.Errorf("UpdateFromLine() JobID = %v, want %v", u.JobID, tt.jobID)
			}

			if u.Stage != tt.wantStage {
				t.Errorf("UpdateFromLine() Stage = %v, want %v", u.Stage, tt.wantStage)
			}

			if u.Percent != tt.wantPercent {
				t.Errorf("UpdateFromLine() Percent = %v, want %v", u.Percent, tt.wantPercent)
			}
		})
	}
}

func TestProgressState_StateTracking(t *testing.T) {
	ps := &ProgressState{}

	// Process state updates
	ps.UpdateFromLine("out_time_ms=15000000", "job1", 60.0, false)
	if ps.OutTimeMs != 15000000 {
		t.Errorf("OutTimeMs = %v, want 15000000", ps.OutTimeMs)
	}

	ps.UpdateFromLine("speed=1.2x", "job1", 60.0, false)
	if ps.SpeedStr != "1.2x" {
		t.Errorf("SpeedStr = %v, want '1.2x'", ps.SpeedStr)
	}

	ps.UpdateFromLine("total_size=1048576", "job1", 60.0, false)
	if ps.TotalSize != 1048576 {
		t.Errorf("TotalSize = %v, want 1048576", ps.TotalSize)
	}
}