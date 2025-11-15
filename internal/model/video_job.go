package model

// QualityPreset represents a named quality configuration.
type QualityPreset string

const (
	PresetLow    QualityPreset = "low"
	PresetMedium QualityPreset = "medium"
	PresetHigh   QualityPreset = "high"
)

// CaptionMode controls writing of captions next to the output file.
type CaptionMode string

const (
	CaptionTxt  CaptionMode = "txt"
	CaptionNone CaptionMode = "none"
)

// CLIOptions holds user-configurable runtime options as parsed from flags.
type CLIOptions struct {
	OutDir     string
	MaxSizeMB  int           // 0 disables size mode and forces CRF mode.
	Quality    QualityPreset // low | medium | high
	Resolution int           // Desired long-side resolution. 0 = use preset default.
	AudioOnly  bool
	Caption    CaptionMode // txt | none
	KeepTemp   bool
	DLBinary   string // Optional explicit path to yt-dlp/youtube-dl
	DryRun     bool
	Verbose    bool

	NoUI bool // Disable TUI when true
	Jobs int  // Max concurrent jobs for TUI
}

// DownloadedVideo represents the media and metadata returned by the downloader.
type DownloadedVideo struct {
	InputPath   string  // Full path to downloaded media file, empty for metadata-only.
	DurationSec float64 // Seconds; may be 0 if unknown.
	Title       string
	Uploader    string
	ID          string
	Description string
	Width       int // 0 if unknown
	Height      int // 0 if unknown
	URL         string
}

// EncodeOptions controls ffmpeg encoding strategy.
type EncodeOptions struct {
	LongSidePx       int    // Desired long-side resolution in pixels.
	ModeCRF          bool   // If true, use CRF; else size-constrained bitrate mode.
	CRF              int    // CRF value for quality mode.
	MaxSizeMB        int    // Target max size for size-constrained mode.
	AudioBitrateKbps int    // Audio bitrate in kbps.
	VideoMinKbps     int    // Clamp lower bound for video bitrate.
	VideoMaxKbps     int    // Clamp upper bound for video bitrate.
	Preset           string // x264 preset, e.g., "veryfast".
	Profile          string // H.264 profile, e.g., "main".
	AudioOnly        bool   // Extract audio only.
	KeyInt           int    // GOP size; 0 to omit.
}

// OutputVideo captures encoding results.
type OutputVideo struct {
	OutputPath      string
	Bytes           int64
	UsedCRF         int // 0 if bitrate mode
	UsedBitrateKbps int // 0 if CRF mode
	LongSidePx      int
	AudioOnly       bool
}

// VideoJob represents a single URL processing job with runtime-resolved paths.
type VideoJob struct {
	URL        string
	CLIOpts    CLIOptions
	YTBinary   string // Path to yt-dlp or youtube-dl
	FFmpegPath string // Path to ffmpeg
	TempDir    string // Per-job temp directory
}
