package util

import (
	"fmt"
	"net/url"
	"strings"
)

type Platform string

const (
	PlatformInstagram Platform = "instagram"
	PlatformYouTube   Platform = "youtube"
)

// DetectPlatform parses a raw URL string and determines if it targets a
// supported platform (Instagram or YouTube). It returns the detected
// platform, the parsed URL, or an error with a clear message if unsupported.
func DetectPlatform(raw string) (Platform, *url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("invalid URL %q", raw)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", nil, fmt.Errorf("invalid URL %q", raw)
	}

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")

	switch host {
	case "instagram.com", "instagr.am":
		return PlatformInstagram, u, nil
	case "youtube.com", "m.youtube.com", "youtu.be":
		return PlatformYouTube, u, nil
	default:
		return "", nil, fmt.Errorf(
			"unsupported URL %q: only Instagram or YouTube are supported (instagram.com, instagr.am, youtube.com, youtu.be)",
			raw,
		)
	}
}