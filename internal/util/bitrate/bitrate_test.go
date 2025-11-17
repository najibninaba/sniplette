package bitrate

import "testing"

func TestComputeVideoKbps(t *testing.T) {
	tests := []struct {
		name        string
		maxSizeMB   int
		durationSec float64
		audioKbps   int
		vMinKbps    int
		vMaxKbps    int
		want        int
	}{
		{
			name:        "normal case - 50MB for 60s video (unclamped)",
			maxSizeMB:   50,
			durationSec: 60.0,
			audioKbps:   128,
			vMinKbps:    500,
			vMaxKbps:    10000,
			want:        6862, // int(((50*1024*1024*8)/60)/1000) - 128
		},
		{
			name:        "zero duration returns max",
			maxSizeMB:   50,
			durationSec: 0,
			audioKbps:   128,
			vMinKbps:    500,
			vMaxKbps:    5000,
			want:        5000,
		},
		{
			name:        "negative duration returns max",
			maxSizeMB:   50,
			durationSec: -1.0,
			audioKbps:   128,
			vMinKbps:    500,
			vMaxKbps:    5000,
			want:        5000,
		},
		{
			name:        "calculated below min - clamps to min",
			maxSizeMB:   1,
			durationSec: 120.0,
			audioKbps:   128,
			vMinKbps:    500,
			vMaxKbps:    5000,
			want:        500,
		},
		{
			name:        "calculated above max - clamps to max",
			maxSizeMB:   500,
			durationSec: 10.0,
			audioKbps:   128,
			vMinKbps:    500,
			vMaxKbps:    5000,
			want:        5000,
		},
		{
			name:        "large file short duration - clamped to 10k",
			maxSizeMB:   100,
			durationSec: 30.0,
			audioKbps:   192,
			vMinKbps:    1000,
			vMaxKbps:    10000,
			want:        10000, // Very high computed bitrate, clamped to max
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeVideoKbps(tt.maxSizeMB, tt.durationSec, tt.audioKbps, tt.vMinKbps, tt.vMaxKbps)
			if got != tt.want {
				t.Errorf("ComputeVideoKbps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name string
		v    int
		min  int
		max  int
		want int
	}{
		{name: "value in range", v: 50, min: 0, max: 100, want: 50},
		{name: "value below min", v: -10, min: 0, max: 100, want: 0},
		{name: "value above max", v: 150, min: 0, max: 100, want: 100},
		{name: "value equals min", v: 0, min: 0, max: 100, want: 0},
		{name: "value equals max", v: 100, min: 0, max: 100, want: 100},
		{name: "negative range", v: -50, min: -100, max: -10, want: -50},
		{name: "single value range", v: 50, min: 42, max: 42, want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clamp(tt.v, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("Clamp(%d, %d, %d) = %v, want %v", tt.v, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestSafeAudioKbps(t *testing.T) {
	tests := []struct {
		name string
		v    int
		want int
	}{
		{name: "below minimum", v: 32, want: 64},
		{name: "at minimum", v: 64, want: 64},
		{name: "above minimum", v: 128, want: 128},
		{name: "zero", v: 0, want: 64},
		{name: "negative", v: -10, want: 64},
		{name: "very high", v: 320, want: 320},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeAudioKbps(tt.v)
			if got != tt.want {
				t.Errorf("SafeAudioKbps(%d) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}