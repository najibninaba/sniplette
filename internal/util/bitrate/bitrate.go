package bitrate

// ComputeVideoKbps calculates the video bitrate (kbps) required to fit maxSizeMB
// given the duration and audio bitrate, clamped between vMinKbps and vMaxKbps.
func ComputeVideoKbps(maxSizeMB int, durationSec float64, audioKbps, vMinKbps, vMaxKbps int) int {
	if durationSec <= 0 {
		return vMaxKbps
	}
	maxBytes := int64(maxSizeMB) * 1024 * 1024
	totalKbps := int((float64(maxBytes*8) / durationSec) / 1000)
	videoKbps := totalKbps - audioKbps
	return Clamp(videoKbps, vMinKbps, vMaxKbps)
}

// Clamp returns v constrained to [min, max].
func Clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// SafeAudioKbps ensures audio bitrate is at least 64 kbps.
func SafeAudioKbps(v int) int {
	if v < 64 {
		return 64
	}
	return v
}