package media

import (
	"fmt"
	"strings"

	"ig2wa/internal/model"
	"ig2wa/internal/util"
)

// OutputBasename builds a safe, informative base filename (without extension)
// derived from metadata and encoding options.
func OutputBasename(dv model.DownloadedVideo, longSide int, maxSizeMB int, enc model.EncodeOptions) string {
	uploader := dv.Uploader
	if uploader == "" {
		uploader = "ig"
	}
	id := dv.ID
	if id == "" {
		id = dv.Title
	}
	uploader = util.SanitizeFilename(uploader)
	id = util.SanitizeFilename(id)

	parts := []string{uploader, id}
	if enc.AudioOnly {
		parts = append(parts, "audio")
	} else {
		parts = append(parts, fmt.Sprintf("%dp", longSide))
		if enc.ModeCRF {
			parts = append(parts, fmt.Sprintf("CRF%d", enc.CRF))
		} else if maxSizeMB > 0 {
			parts = append(parts, fmt.Sprintf("%dMB", maxSizeMB))
		}
	}
	return strings.Join(parts, "_")
}

// CaptionText renders a caption text with title/uploader/url and description.
func CaptionText(dv model.DownloadedVideo) string {
	var b strings.Builder
	title := strings.TrimSpace(dv.Title)
	uploader := strings.TrimSpace(dv.Uploader)
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	}
	if uploader != "" {
		b.WriteString(uploader)
		b.WriteString("\n")
	}
	if dv.URL != "" {
		b.WriteString(dv.URL)
		b.WriteString("\n")
	}
	b.WriteString("\n---\nORIGINAL CAPTION\n")
	if dv.Description != "" {
		b.WriteString(dv.Description)
		b.WriteString("\n")
	}
	return b.String()
}