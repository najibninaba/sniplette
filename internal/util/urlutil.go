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
	PlatformThreads   Platform = "threads"
)

// DetectPlatform parses a raw URL string and determines if it targets a
// supported platform (Instagram, YouTube, or Threads). It returns the detected
// platform, the parsed URL, or an error with a clear message if unsupported.
func DetectPlatform(raw string) (Platform, *url.URL, error) {
	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "" || u.Host == "") {
		if u2, e2 := url.Parse("https://" + raw); e2 == nil {
			u = u2
		}
	}
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", nil, fmt.Errorf("invalid URL %q", raw)
	}

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")

	switch host {
	case "instagram.com", "instagr.am", "m.instagram.com":
		return PlatformInstagram, u, nil
	case "youtube.com", "m.youtube.com", "youtu.be":
		return PlatformYouTube, u, nil
	case "threads.net", "threads.com":
		return "", nil, fmt.Errorf("unsupported URL %q: Threads is not currently supported (yt-dlp has no extractor). Use Instagram or YouTube.", raw)
	default:
		return "", nil, fmt.Errorf(
			"unsupported URL %q: only Instagram or YouTube are supported (instagram.com, instagr.am, youtube.com, youtu.be)",
			raw,
		)
	}
}

// NormalizeURL normalizes service-specific URLs for compatibility with external tools.
// For PlatformThreads, convert any threads.com host (and subdomains) to threads.net.
// For other platforms, the URL is returned unchanged.
func NormalizeURL(raw string, platform Platform) string {
	if platform != PlatformThreads {
		return raw
	}

	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "" || u.Host == "") {
		if u2, e2 := url.Parse("https://" + raw); e2 == nil {
			u = u2
		}
	}
	if u == nil || u.Host == "" {
		return raw
	}

	lowerHost := strings.ToLower(u.Host)
	if strings.HasSuffix(lowerHost, "threads.com") {
		prefix := u.Host[:len(u.Host)-len("threads.com")]
		u.Host = prefix + "threads.net"
		return u.String()
	}
	return raw
}