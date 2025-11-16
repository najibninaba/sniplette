package deps

import (
	"fmt"
	"os"
	"os/exec"
)

// FindDownloader returns the path to yt-dlp or youtube-dl.
// If customPath is non-empty, it tries that path or looks it up in PATH.
func FindDownloader(customPath string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
		if p, err := exec.LookPath(customPath); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("could not find downloader at %q", customPath)
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("youtube-dl"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find yt-dlp or youtube-dl in PATH. Please install yt-dlp.")
}

// FindFFmpeg returns the path to the ffmpeg binary in PATH.
func FindFFmpeg() (string, error) {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find ffmpeg in PATH. Please install ffmpeg.")
}