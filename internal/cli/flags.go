package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ig2wa/internal/model"
	"ig2wa/internal/util"
)

// Parsed is the result of ParseFlags.
type Parsed struct {
	URLs    []string
	Options model.CLIOptions
	// PresetCRF is the CRF mapped from the chosen preset (used by caller).
	PresetCRF int
}

// ParseFlags parses CLI flags and arguments into a structured form.
func ParseFlags(_ context.Context, args []string) (Parsed, error) {
	fs := flag.NewFlagSet("ig2wa", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var outDir string
	var maxSizeMB int
	var quality string
	var resolution int
	var audioOnly bool
	var caption string
	var keepTemp bool
	var dlBinary string
	var dryRun bool
	var verbose bool
	// New UI-related flags
	var noUI bool
	var jobs int

	fs.StringVar(&outDir, "out-dir", ".", "Output directory")
	fs.StringVar(&outDir, "o", ".", "Output directory (shorthand)")
	fs.IntVar(&maxSizeMB, "max-size-mb", 50, "Target max size per video (MB). Set 0 to use CRF mode.")
	fs.StringVar(&quality, "quality-preset", "medium", "Quality preset: low, medium, high")
	fs.IntVar(&resolution, "resolution", 0, "Override long-side resolution in px (e.g., 540, 720, 1080); 0 uses preset default")
	fs.BoolVar(&audioOnly, "audio-only", false, "Extract audio only (M4A)")
	fs.StringVar(&caption, "caption", "txt", "Caption output: txt, none")
	fs.BoolVar(&keepTemp, "keep-temp", false, "Keep intermediate downloads")
	fs.StringVar(&dlBinary, "dl-binary", "", "Path to yt-dlp or youtube-dl")
	fs.BoolVar(&dryRun, "dry-run", false, "Show plan without executing")
	fs.BoolVar(&verbose, "verbose", false, "Show full subprocess commands/output")
	fs.BoolVar(&verbose, "v", false, "Show full subprocess commands/output (shorthand)")
	// Define new flags
	fs.BoolVar(&noUI, "no-ui", false, "Disable TUI; use plain textual output")
	fs.IntVar(&jobs, "jobs", 2, "Max concurrent jobs in TUI")

	if err := fs.Parse(args); err != nil {
		return Parsed{}, err
	}

	rem := fs.Args()
	if len(rem) == 0 {
		return Parsed{}, errors.New("usage: ig2wa <url> [<url> ...] [flags]\nsupported: Instagram (instagram.com, instagr.am) and YouTube (youtube.com, youtu.be)")
	}

	quality = strings.ToLower(quality)
	if quality != string(model.PresetLow) && quality != string(model.PresetMedium) && quality != string(model.PresetHigh) {
		return Parsed{}, fmt.Errorf("invalid --quality-preset: %q (valid: low|medium|high)", quality)
	}

	caption = strings.ToLower(caption)
	if caption != string(model.CaptionTxt) && caption != string(model.CaptionNone) {
		return Parsed{}, fmt.Errorf("invalid --caption: %q (valid: txt|none)", caption)
	}

	var urls []string
	for _, raw := range rem {
		if _, _, err := util.DetectPlatform(raw); err != nil {
			return Parsed{}, err
		}
		urls = append(urls, raw)
	}

	// Resolve defaults based on preset
	preset := model.QualityPreset(quality)
	presetRes, presetMaxMB, presetCRF := presetDefaults(preset)

	if resolution <= 0 {
		resolution = presetRes
	}
	// If explicitly set to 0, CRF mode will be used; otherwise use provided max or preset.
	if flagIsPresent(args, "--max-size-mb") && maxSizeMB == 0 {
		// User asked to disable size mode
	} else if !flagIsPresent(args, "--max-size-mb") {
		maxSizeMB = presetMaxMB
	}

	if dlBinary == "" {
		dlBinary = os.Getenv("IG2WA_DL_BINARY")
	}

	// Normalize output dir path
	if outDir == "" {
		outDir = "."
	}
	outDir = filepath.Clean(outDir)

	if jobs <= 0 {
		jobs = 2
	}

	opts := model.CLIOptions{
		OutDir:     outDir,
		MaxSizeMB:  maxSizeMB,
		Quality:    preset,
		Resolution: resolution,
		AudioOnly:  audioOnly,
		Caption:    model.CaptionMode(caption),
		KeepTemp:   keepTemp,
		DLBinary:   dlBinary,
		DryRun:     dryRun,
		Verbose:    verbose,
		NoUI:       noUI,
		Jobs:       jobs,
	}

	return Parsed{
		URLs:      urls,
		Options:   opts,
		PresetCRF: presetCRF,
	}, nil
}

func validateInstagramURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL: %q", raw)
	}
	host := strings.ToLower(u.Host)
	if !strings.Contains(host, "instagram.com") && !strings.Contains(host, "instagr.am") {
		return fmt.Errorf("this URL does not look like Instagram: %q", raw)
	}
	return nil
}

func presetDefaults(p model.QualityPreset) (resolution int, maxSizeMB int, crf int) {
	switch p {
	case model.PresetLow:
		return 540, 20, 26
	case model.PresetHigh:
		return 1080, 100, 19
	case model.PresetMedium:
		fallthrough
	default:
		return 720, 50, 22
	}
}

func flagIsPresent(args []string, name string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, name) {
			return true
		}
	}
	return false
}
