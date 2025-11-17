package encoder

import (
	"strconv"
	"strings"

	"ig2wa/internal/progress"
)

// ParseProgressLine parses a single line from ffmpeg's -progress output.
// Returns a progress.Update if the line contains useful progress info, and ok=true.
func ParseProgressLine(line string, jobID string, durationSec float64, isAudioOnly bool) (u progress.Update, ok bool) {
	kv := strings.SplitN(line, "=", 2)
	if len(kv) != 2 {
		return progress.Update{}, false
	}

	key := strings.TrimSpace(kv[0])
	val := strings.TrimSpace(kv[1])

	switch key {
	case "progress":
		// Emit progress update - caller should compute percent from state
		msg := "Encoding"
		if isAudioOnly {
			msg = "Encoding (audio)"
		}
		return progress.Update{
			JobID:   jobID,
			Stage:   progress.StageEncoding,
			Percent: -1,
			Message: msg,
		}, true
	}

	_ = val // unused here; stateful parsing handled via ProgressState
	return progress.Update{}, false
}

// ProgressState helps track progress across multiple line parses.
type ProgressState struct {
	OutTimeMs int64
	SpeedStr  string
	TotalSize int64
}

// UpdateFromLine updates the state from a progress line and returns an update if progress marker found.
func (ps *ProgressState) UpdateFromLine(line string, jobID string, durationSec float64, isAudioOnly bool) (u progress.Update, ok bool) {
	kv := strings.SplitN(line, "=", 2)
	if len(kv) != 2 {
		return progress.Update{}, false
	}

	key := strings.TrimSpace(kv[0])
	val := strings.TrimSpace(kv[1])

	switch key {
	case "out_time_ms":
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			ps.OutTimeMs = v
		}
	case "speed":
		ps.SpeedStr = val
	case "total_size":
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			ps.TotalSize = v
		}
	case "progress":
		// Emit progress update
		percent := -1.0
		if !isAudioOnly && durationSec > 0 {
			den := durationSec * 1_000_000 // out_time_ms uses microseconds
			if den > 0 {
				percent = (float64(ps.OutTimeMs) / den) * 100.0
				if percent > 100 {
					percent = 100
				}
			}
		}

		var speedPtr *string
		if ps.SpeedStr != "" {
			s := ps.SpeedStr
			speedPtr = &s
		}

		var bytesPtr *int64
		if ps.TotalSize > 0 {
			b := ps.TotalSize
			bytesPtr = &b
		}

		msg := "Encoding"
		if isAudioOnly {
			msg = "Encoding (audio)"
		}

		return progress.Update{
			JobID:   jobID,
			Stage:   progress.StageEncoding,
			Percent: percent,
			Speed:   speedPtr,
			Bytes:   bytesPtr,
			Message: msg,
		}, true
	}

	return progress.Update{}, false
}