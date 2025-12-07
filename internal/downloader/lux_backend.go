package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ig2wa/internal/model"
	"ig2wa/internal/progress"
	"ig2wa/internal/util"

	"github.com/iawia002/lux/extractors"
	// Import extractors to register them
	_ "github.com/iawia002/lux/extractors/facebook"
	_ "github.com/iawia002/lux/extractors/threads"
	_ "github.com/iawia002/lux/extractors/tiktok"
	_ "github.com/iawia002/lux/extractors/twitter"
)

// downloadViaLux uses lux extractors to download video from supported platforms.
func downloadViaLux(ctx context.Context, url, workdir string, opts Options, platform util.Platform) (model.DownloadedVideo, string, error) {
	// Extract video data using lux
	luxOpts := extractors.Options{
		Cookie: "", // TODO: support cookie passing if needed
	}

	dataList, err := extractors.Extract(url, luxOpts)
	if err != nil {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("%w: [%s] extraction failed: %v", ErrLuxFailed, platform, err)
	}

	if len(dataList) == 0 {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("%w: [%s] no data extracted", ErrLuxFailed, platform)
	}

	// Use first data entry
	data := dataList[0]
	if data.Err != nil {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("%w: [%s] %v", ErrLuxFailed, platform, data.Err)
	}

	// Fill up stream data (calculates sizes, etc.)
	data.FillUpStreamsData()

	// Select best stream
	stream := selectBestStream(data)
	if stream == nil {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("%w: [%s] no streams available", ErrLuxFailed, platform)
	}

	// Build metadata for DownloadedVideo
	dv := luxDataToDownloadedVideo(data, stream, url)

	// If metadata only, return early
	if opts.MetadataOnly {
		return dv, workdir, nil
	}

	// Download the stream
	if opts.Reporter != nil {
		opts.Reporter.Update(progress.Update{
			JobID:   opts.JobID,
			Stage:   progress.StageDownloading,
			Percent: 0,
			Message: "Downloading via Lux",
		})
	}

	inputPath, err := downloadLuxStream(ctx, data, stream, workdir, opts, platform)
	if err != nil {
		return model.DownloadedVideo{}, workdir, fmt.Errorf("%w: [%s] download failed: %v", ErrLuxFailed, platform, err)
	}

	dv.InputPath = inputPath
	return dv, workdir, nil
}

// selectBestStream picks the highest quality stream from the data.
func selectBestStream(data *extractors.Data) *extractors.Stream {
	if len(data.Streams) == 0 {
		return nil
	}

	// Try to find "default" stream first
	if s, ok := data.Streams["default"]; ok {
		return s
	}

	// Otherwise pick the stream with the largest size (usually best quality)
	var best *extractors.Stream
	var bestSize int64
	for _, s := range data.Streams {
		if s.Size > bestSize {
			best = s
			bestSize = s.Size
		}
	}

	// If no sizes available, just pick any stream
	if best == nil {
		for _, s := range data.Streams {
			return s
		}
	}

	return best
}

// luxDataToDownloadedVideo maps lux Data to sniplette's DownloadedVideo model.
func luxDataToDownloadedVideo(data *extractors.Data, stream *extractors.Stream, url string) model.DownloadedVideo {
	// Extract ID from URL or title
	id := extractIDFromURL(url)
	if id == "" {
		id = util.SanitizeFilename(data.Title)
		if len(id) > 20 {
			id = id[:20]
		}
	}

	// Use site name as uploader if not available
	uploader := data.Site
	if uploader == "" {
		uploader = "unknown"
	}

	// Try to extract dimensions from stream quality string (e.g., "720p", "1080x1920")
	width, height := parseStreamDimensions(stream)

	return model.DownloadedVideo{
		InputPath:   "", // Set after download
		DurationSec: 0,  // Lux doesn't reliably provide duration
		Title:       data.Title,
		Uploader:    uploader,
		ID:          id,
		Description: "", // Not available from lux
		Width:       width,
		Height:      height,
		URL:         url,
	}
}

// parseStreamDimensions tries to extract width/height from stream quality string.
func parseStreamDimensions(stream *extractors.Stream) (width, height int) {
	if stream == nil {
		return 0, 0
	}

	quality := stream.Quality
	if quality == "" {
		quality = stream.ID
	}

	// Try to parse "WIDTHxHEIGHT" format (e.g., "1920x1080")
	if strings.Contains(quality, "x") {
		parts := strings.Split(quality, "x")
		if len(parts) == 2 {
			if w, err := parseInt(parts[0]); err == nil {
				if h, err := parseInt(parts[1]); err == nil {
					return w, h
				}
			}
		}
	}

	// Try to parse "XXXp" format (e.g., "720p", "1080p")
	if strings.HasSuffix(strings.ToLower(quality), "p") {
		pStr := strings.TrimSuffix(strings.ToLower(quality), "p")
		// Extract just the number part
		numStr := ""
		for _, c := range pStr {
			if c >= '0' && c <= '9' {
				numStr += string(c)
			} else {
				numStr = "" // Reset if non-digit found before digits
			}
		}
		if numStr != "" {
			if h, err := parseInt(numStr); err == nil && h > 0 {
				// Assume 16:9 aspect ratio for height-only
				return h * 16 / 9, h
			}
		}
	}

	return 0, 0
}

// parseInt parses a string to int, returning error if invalid.
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// extractIDFromURL tries to extract a video ID from the URL.
func extractIDFromURL(url string) string {
	// Simple extraction based on common patterns
	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		// Skip empty parts and query strings
		if p == "" || strings.HasPrefix(p, "?") {
			continue
		}
		// Remove query string from the part
		if idx := strings.Index(p, "?"); idx > 0 {
			p = p[:idx]
		}
		// Return if it looks like an ID (alphanumeric, reasonable length)
		if len(p) >= 5 && len(p) <= 50 {
			return p
		}
	}
	return ""
}

// downloadLuxStream downloads the stream parts to the workdir.
func downloadLuxStream(ctx context.Context, data *extractors.Data, stream *extractors.Stream, workdir string, opts Options, platform util.Platform) (string, error) {
	if len(stream.Parts) == 0 {
		return "", fmt.Errorf("no parts in stream")
	}

	// Fail fast on multi-part streams (HLS, segmented) - not yet supported
	if len(stream.Parts) > 1 {
		return "", fmt.Errorf("multi-part streams not supported yet (got %d parts); try a different quality or source", len(stream.Parts))
	}

	// Single part - download directly
	part := stream.Parts[0]
	ext := part.Ext
	if ext == "" {
		ext = "mp4"
	}

	// Build output filename
	filename := util.SanitizeFilename(data.Title)
	if filename == "" {
		filename = "video"
	}
	if len(filename) > 100 {
		filename = filename[:100]
	}
	outputPath := filepath.Join(workdir, filename+"."+ext)

	err := downloadFile(ctx, part.URL, outputPath, part.Size, opts)
	if err != nil {
		return "", err
	}
	return outputPath, nil
}

// downloadFile downloads a file from URL to the given path with progress reporting.
func downloadFile(ctx context.Context, url, outputPath string, expectedSize int64, opts Options) error {
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set a reasonable user agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Minute, // Long timeout for large files
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status %d: %s", resp.StatusCode, resp.Status)
	}

	// Create output file
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	// Determine total size
	totalSize := expectedSize
	if totalSize <= 0 {
		totalSize = resp.ContentLength
	}

	// Create progress writer if reporter is available
	var writer io.Writer = out
	if opts.Reporter != nil {
		writer = &progressWriter{
			writer:     out,
			reporter:   opts.Reporter,
			jobID:      opts.JobID,
			totalSize:  totalSize,
			lastUpdate: time.Now(),
		}
	}

	// Copy data
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Final progress update
	if opts.Reporter != nil {
		fi, _ := out.Stat()
		size := fi.Size()
		opts.Reporter.Update(progress.Update{
			JobID:   opts.JobID,
			Stage:   progress.StageDownloading,
			Percent: 100,
			Bytes:   &size,
			Message: "Download complete",
		})
	}

	return nil
}

// progressWriter wraps an io.Writer and reports progress.
type progressWriter struct {
	writer     io.Writer
	reporter   progress.Reporter
	jobID      string
	totalSize  int64
	downloaded int64
	lastUpdate time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}

	pw.downloaded += int64(n)

	// Throttle updates to every 200ms
	if time.Since(pw.lastUpdate) > 200*time.Millisecond {
		var percent float64 = -1
		if pw.totalSize > 0 {
			percent = float64(pw.downloaded) / float64(pw.totalSize) * 100
		}

		downloaded := pw.downloaded
		pw.reporter.Update(progress.Update{
			JobID:   pw.jobID,
			Stage:   progress.StageDownloading,
			Percent: percent,
			Bytes:   &downloaded,
			Message: "Downloading",
		})
		pw.lastUpdate = time.Now()
	}

	return n, nil
}
