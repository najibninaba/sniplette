package downloader

import (
	"strconv"
	"strings"
	"time"

	"ig2wa/internal/progress"
)

// ParseProgress parses yt-dlp progress output lines.
// Returns a progress.Update if the line contains download progress, and ok=true.
func ParseProgress(line, jobID string) (u progress.Update, ok bool) {
	// yt-dlp outputs lines like: [download]  45.2% of 10.00MiB at  1.50MiB/s ETA 00:04
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[download]") {
		return progress.Update{}, false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(line, "[download]"))

	// Parse percent
	var percent float64 = -1
	if idx := strings.Index(rest, "%"); idx != -1 {
		pctStr := strings.TrimSpace(rest[:idx])
		if p, err := strconv.ParseFloat(pctStr, 64); err == nil {
			percent = p
		}
	}

	// Parse speed (e.g., "at 1.50MiB/s")
	var speed *string
	if idx := strings.Index(rest, " at "); idx != -1 {
		speedPart := rest[idx+4:]
		if idx2 := strings.Index(speedPart, " "); idx2 != -1 {
			s := strings.TrimSpace(speedPart[:idx2])
			speed = &s
		}
	}

	// Parse ETA (e.g., "ETA 00:04")
	var eta *time.Duration
	if idx := strings.Index(rest, "ETA "); idx != -1 {
		etaStr := strings.TrimSpace(rest[idx+4:])
		if idx2 := strings.Index(etaStr, " "); idx2 != -1 {
			etaStr = etaStr[:idx2]
		}
		if d, err := parseETA(etaStr); err == nil {
			eta = &d
		}
	}

	return progress.Update{
		JobID:   jobID,
		Stage:   progress.StageDownloading,
		Percent: percent,
		Speed:   speed,
		ETA:     eta,
		Message: "Downloading",
	}, true
}

// parseETA parses duration strings like "00:04", "01:23:45", etc.
func parseETA(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		// MM:SS
		m, err1 := strconv.Atoi(parts[0])
		sec, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, err1
		}
		return time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	case 3:
		// HH:MM:SS
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		sec, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, err1
		}
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	default:
		// Try parsing as seconds
		sec, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return time.Duration(sec) * time.Second, nil
	}
}