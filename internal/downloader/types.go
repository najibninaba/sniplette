package downloader

// YTDLPInfo mirrors fields from yt-dlp --dump-json output that we care about.
type YTDLPInfo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Uploader    string  `json:"uploader"`
	Duration    float64 `json:"duration"`
	Description string  `json:"description"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
}
