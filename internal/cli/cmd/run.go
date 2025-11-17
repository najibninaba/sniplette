package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"ig2wa/internal/downloader"
	"ig2wa/internal/encoder"
	"ig2wa/internal/model"
	"ig2wa/internal/pipeline"
	"ig2wa/internal/ui"
	"ig2wa/internal/util"
	"ig2wa/internal/util/deps"
	"ig2wa/internal/util/media"
)

type runMode struct {
	ForceTUI   bool
	DryRunOnly bool
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "run [urls...]",
		Short:         "Run fetch/encode pipeline for tiny snips",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		PreRunE:       runPreRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecute(cmd, args, runMode{
				ForceTUI:   false,
				DryRunOnly: false,
			})
		},
	}
	// Bind same flags as root for explicit subcommand usage
	bindRunFlags(cmd.Flags())
	_ = cmd.Flags().MarkDeprecated("dry-run", "use 'sniplette plan' instead")
	_ = cmd.Flags().MarkDeprecated("no-ui", "use 'sniplette tui' for interactive mode")
	return cmd
}

type ctxKey string

const runInputsKey ctxKey = "runInputs"

type runInputs struct {
	URLs      []string
	Options   model.CLIOptions
	PresetCRF int
}

func runPreRun(cmd *cobra.Command, args []string) error {
	urls, opts, presetCRF, err := assembleRunInputs(cmd, args)
	if err != nil {
		return &ExitError{Code: ExitCLIError, Err: err}
	}
	ctx := context.WithValue(cmd.Context(), runInputsKey, runInputs{
		URLs:      urls,
		Options:   opts,
		PresetCRF: presetCRF,
	})
	cmd.SetContext(ctx)
	return nil
}

func assembleRunInputs(cmd *cobra.Command, args []string) ([]string, model.CLIOptions, int, error) {
	// Persistent flags with precedence: flag > env/config > default
	defaultOut := "."
	outDir := getPersistentString(cmd, "out-dir", defaultOut)
	verbose := getPersistentBool(cmd, "verbose", false)
	dlBinary := getPersistentString(cmd, "dl-binary", "")
	jobs := getPersistentInt(cmd, "jobs", 2)
	if jobs <= 0 {
		jobs = 2
	}

	// Run flags
	maxSizeMB, _ := cmd.Flags().GetInt("max-size-mb")
	quality, _ := cmd.Flags().GetString("quality-preset")
	resolution, _ := cmd.Flags().GetInt("resolution")
	audioOnly, _ := cmd.Flags().GetBool("audio-only")
	caption, _ := cmd.Flags().GetString("caption")
	keepTemp, _ := cmd.Flags().GetBool("keep-temp")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	noUI, _ := cmd.Flags().GetBool("no-ui")

	quality = strings.ToLower(quality)
	switch quality {
	case string(model.PresetLow), string(model.PresetMedium), string(model.PresetHigh):
	default:
		return nil, model.CLIOptions{}, 0, fmt.Errorf("invalid --quality-preset: %q (valid: low|medium|high)", quality)
	}

	caption = strings.ToLower(caption)
	if caption != string(model.CaptionTxt) && caption != string(model.CaptionNone) {
		return nil, model.CLIOptions{}, 0, fmt.Errorf("invalid --caption: %q (valid: txt|none)", caption)
	}

	// URL validation
	var urls []string
	for _, raw := range args {
		if _, _, err := util.DetectPlatform(raw); err != nil {
			return nil, model.CLIOptions{}, 0, err
		}
		urls = append(urls, raw)
	}

	// Defaults based on preset
	preset := model.QualityPreset(quality)
	presetRes, presetMaxMB, presetCRF := presetDefaults(preset)

	if resolution <= 0 {
		resolution = presetRes
	}
	changedMax := cmd.Flags().Changed("max-size-mb")
	if !changedMax {
		maxSizeMB = presetMaxMB
	} else if maxSizeMB < 0 {
		maxSizeMB = 0
	}

	if dlBinary == "" {
		if v := os.Getenv("SNIPLETTE_DL_BINARY"); v != "" {
			dlBinary = v
		} else if v := os.Getenv("IG2WA_DL_BINARY"); v != "" { // backward-compatibility fallback
			dlBinary = v
		}
	}

	outDir = filepath.Clean(outDir)

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
	return urls, opts, presetCRF, nil
}

func runExecute(cmd *cobra.Command, args []string, mode runMode) error {
	// Grab inputs from context; if not present (root directly called without PreRunE), assemble now.
	var in runInputs
	if v := cmd.Context().Value(runInputsKey); v != nil {
		in = v.(runInputs)
	} else {
		urls, opts, presetCRF, err := assembleRunInputs(cmd, args)
		if err != nil {
			return &ExitError{Code: ExitCLIError, Err: err}
		}
		in = runInputs{URLs: urls, Options: opts, PresetCRF: presetCRF}
	}

	// Ensure output directory exists early when using TUI
	if err := ensureDir(in.Options.OutDir); err != nil {
		return &ExitError{Code: ExitCLIError, Err: fmt.Errorf("failed to create output dir: %v", err)}
	}

	// TUI path (forced or auto if TTY and not disabled)
	useTUI := mode.ForceTUI || (!in.Options.NoUI && isTerminal())
	if useTUI && !mode.DryRunOnly {
		if err := ui.Run(cmd.Context(), in.URLs, in.Options); err != nil {
			return &ExitError{Code: ExitCLIError, Err: err}
		}
		return nil
	}

	// Non-UI path
	downloaderPath, derr := deps.FindDownloader(in.Options.DLBinary)
	if derr != nil {
		return &ExitError{Code: ExitMissingDep, Err: derr}
	}
	ffmpegPath, ferr := deps.FindFFmpeg()
	if ferr != nil {
		return &ExitError{Code: ExitMissingDep, Err: ferr}
	}

	// Ensure output directory exists (again, for non-UI-only invocations)
	if err := ensureDir(in.Options.OutDir); err != nil {
		return &ExitError{Code: ExitCLIError, Err: fmt.Errorf("failed to create output dir: %v", err)}
	}

	// Dry-run-only mode forces metadata-only planning
	if mode.DryRunOnly {
		in.Options.DryRun = true
		in.Options.NoUI = true
	}

	for _, rawURL := range in.URLs {
		if err := processOne(cmd.Context(), rawURL, in, downloaderPath, ffmpegPath); err != nil {
			var ee *ExitError
			if errors.As(err, &ee) {
				return ee
			}
			return &ExitError{Code: ExitCLIError, Err: err}
		}
	}
	return nil
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

var (
	errDownload = errors.New("download failed")
	errEncode   = errors.New("encode failed")
)

func processOne(ctx context.Context, rawURL string, in runInputs, dlPath, ffmpegPath string) error {
	metaOnly := in.Options.DryRun
	dv, tempDir, derr := downloader.Download(ctx, rawURL, downloader.Options{
		DownloaderPath: dlPath,
		Verbose:        in.Options.Verbose,
		KeepTemp:       in.Options.KeepTemp,
		MetadataOnly:   metaOnly,
	})
	defer func() {
		if !in.Options.KeepTemp && tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if derr != nil {
		return &ExitError{Code: ExitDownloadError, Err: fmt.Errorf("%w: %v", errDownload, derr)}
	}

	// Plan encoding
	targetLongSide, crf := pipeline.PlanResolutionAndCRF(in.Options, dv, in.PresetCRF)
	encOpts := model.EncodeOptions{
		LongSidePx:       targetLongSide,
		ModeCRF:          in.Options.MaxSizeMB == 0 || dv.DurationSec <= 0 || in.Options.AudioOnly,
		CRF:              crf,
		MaxSizeMB:        in.Options.MaxSizeMB,
		AudioBitrateKbps: 96,
		VideoMinKbps:     500,
		VideoMaxKbps:     8000,
		Preset:           "veryfast",
		Profile:          "main",
		AudioOnly:        in.Options.AudioOnly,
		KeyInt:           48,
	}

	// Output filename
	base := media.OutputBasename(dv, targetLongSide, in.Options.MaxSizeMB, encOpts)
	ext := ".mp4"
	if in.Options.AudioOnly {
		ext = ".m4a"
	}
	outputPath := filepath.Join(in.Options.OutDir, base+ext)

	if in.Options.DryRun {
		printPlan(rawURL, dlPath, ffmpegPath, tempDir, outputPath, dv, encOpts, in.Options)
		return nil
	}

	// Encode
	out, eerr := encoder.Encode(ctx, dv, encOpts, encoder.Options{
		FFmpegPath: ffmpegPath,
		Verbose:    in.Options.Verbose,
		OutputPath: outputPath,
	})
	if eerr != nil {
		return &ExitError{Code: ExitTranscodeError, Err: fmt.Errorf("%w: %v", errEncode, eerr)}
	}

	// Caption output
	if in.Options.Caption == model.CaptionTxt {
		caption := media.CaptionText(dv)
		if _, werr := util.WriteCaptionFile(out.OutputPath, caption); werr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write caption: %v\n", werr)
		}
	}

	// Size overshoot warning (best-effort)
	if !encOpts.ModeCRF && in.Options.MaxSizeMB > 0 {
		maxBytes := int64(in.Options.MaxSizeMB) * 1024 * 1024
		if out.Bytes > int64(float64(maxBytes)*1.10) {
			fmt.Fprintf(os.Stderr, "warning: output size (%0.2f MB) exceeds target (%d MB). Consider lowering bitrate or preset.\n",
				float64(out.Bytes)/(1024*1024), in.Options.MaxSizeMB)
		}
	}

	fmt.Printf("Saved: %s (%0.2f MB)\n", out.OutputPath, float64(out.Bytes)/(1024*1024))
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

// printPlan outputs a dry-run plan of actions without executing them.
func printPlan(rawURL, dlPath, ffmpegPath, tempDir, outputPath string, dv model.DownloadedVideo, enc model.EncodeOptions, opts model.CLIOptions) {
	fmt.Println("Dry-run plan:")
	fmt.Printf("- URL:            %s\n", rawURL)
	fmt.Printf("- Downloader:     %s\n", dlPath)
	fmt.Printf("- FFmpeg:         %s\n", ffmpegPath)
	fmt.Printf("- Temp dir:       %s\n", tempDir)
	fmt.Printf("- Output dir:     %s\n", opts.OutDir)
	fmt.Printf("- Output path:    %s\n", outputPath)
	fmt.Printf("- Audio only:     %v\n", enc.AudioOnly)
	if !enc.AudioOnly {
		fmt.Printf("- Resolution:     %dp (long side)\n", enc.LongSidePx)
		if enc.ModeCRF {
			fmt.Printf("- Mode:           CRF %d\n", enc.CRF)
		} else {
			kbps := 0
			if dv.DurationSec > 0 && opts.MaxSizeMB > 0 {
				kbps = bitrateForPreview(opts.MaxSizeMB, dv.DurationSec, enc.AudioBitrateKbps, enc.VideoMinKbps, enc.VideoMaxKbps)
			}
			fmt.Printf("- Mode:           Size-constrained (target %d MB), est video bitrate ~ %d kbps\n", opts.MaxSizeMB, kbps)
		}
	} else {
		fmt.Printf("- Audio bitrate:  %d kbps (AAC)\n", enc.AudioBitrateKbps)
	}
	fmt.Printf("- Caption:        %s\n", strings.ToUpper(string(opts.Caption)))
}

func bitrateForPreview(maxSizeMB int, durationSec float64, audioKbps, vMin, vMax int) int {
	if durationSec <= 0 {
		return clamp(2000, vMin, vMax)
	}
	targetSizeBytes := int64(maxSizeMB) * 1024 * 1024
	totalBitrateBps := float64(targetSizeBytes*8) / durationSec
	videoBitrateBps := totalBitrateBps - float64(audioKbps*1000)
	kbps := int(videoBitrateBps / 1000.0)
	return clamp(kbps, vMin, vMax)
}

func clamp(v, min, max int) int {
	if min != 0 && v < min {
		return min
	}
	if max != 0 && v > max {
		return max
	}
	return v
}