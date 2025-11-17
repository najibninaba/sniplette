package downloader

import (
	"testing"
	"time"

	"ig2wa/internal/progress"
)

func TestParseProgress(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		jobID       string
		wantOk      bool
		wantPercent float64
		wantETA     *time.Duration
	}{
		{
			name:        "typical download progress",
			line:        "[download]  45.2% of 10.00MiB at  1.50MiB/s ETA 00:04",
			jobID:       "job1",
			wantOk:      true,
			wantPercent: 45.2,
			wantETA:     durationPtr(4 * time.Second),
		},
		{
			name:        "progress without ETA",
			line:        "[download]  25.0% of 5.00MiB at  500.00KiB/s",
			jobID:       "job2",
			wantOk:      true,
			wantPercent: 25.0,
		},
		{
			name:        "progress with HH:MM:SS ETA",
			line:        "[download]  10.5% of 100.00MiB at  1.00MiB/s ETA 01:23:45",
			jobID:       "job3",
			wantOk:      true,
			wantPercent: 10.5,
			wantETA:     durationPtr(1*time.Hour + 23*time.Minute + 45*time.Second),
		},
		{
			name:   "non-download line",
			line:   "[ExtractorError] Unable to download webpage",
			jobID:  "job4",
			wantOk: false,
		},
		{
			name:   "empty line",
			line:   "",
			jobID:  "job5",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, ok := ParseProgress(tt.line, tt.jobID)

			if ok != tt.wantOk {
				t.Errorf("ParseProgress() ok = %v, want %v", ok, tt.wantOk)
			}

			if !tt.wantOk {
				return
			}

			if u.JobID != tt.jobID {
				t.Errorf("ParseProgress() JobID = %v, want %v", u.JobID, tt.jobID)
			}

			if u.Percent != tt.wantPercent {
				t.Errorf("ParseProgress() Percent = %v, want %v", u.Percent, tt.wantPercent)
			}

			if u.Stage != progress.StageDownloading {
				t.Errorf("ParseProgress() Stage = %v, want StageDownloading", u.Stage)
			}

			if tt.wantETA != nil {
				if u.ETA == nil || *u.ETA != *tt.wantETA {
					t.Errorf("ParseProgress() ETA = %v, want %v", ptrDur(u.ETA), *tt.wantETA)
				}
			}
		})
	}
}

func TestParseETA(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    time.Duration
		wantErr bool
	}{
		{name: "MM:SS format", s: "04:30", want: 4*time.Minute + 30*time.Second},
		{name: "HH:MM:SS format", s: "01:23:45", want: 1*time.Hour + 23*time.Minute + 45*time.Second},
		{name: "seconds only", s: "45", want: 45 * time.Second},
		{name: "zero seconds", s: "00:00", want: 0},
		{name: "one hour exactly", s: "01:00:00", want: 1 * time.Hour},
		{name: "invalid format", s: "invalid", wantErr: true},
		{name: "too many colons", s: "1:2:3:4", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseETA(tt.s)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseETA(%q) expected error, got nil", tt.s)
				}
				return
			}
			if err != nil {
				t.Errorf("parseETA(%q) unexpected error: %v", tt.s, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseETA(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// Helper functions
func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func ptrDur(d *time.Duration) string {
	if d == nil {
		return "<nil>"
	}
	return d.String()
}