package format

import "strconv"

// HumanizeBytes converts a byte count into a human-readable string (e.g., "1.5 MB").
func HumanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return strconv.FormatInt(b, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// Use a fixed buffer to avoid allocation
	var buf [20]byte
	frac := float64(b) / float64(div)
	s := strconv.AppendFloat(buf[:0], frac, 'f', 1, 64)
	suffix := []string{"KB", "MB", "GB", "TB"}[exp]
	return string(s) + " " + suffix
}