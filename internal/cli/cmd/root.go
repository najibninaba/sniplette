package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	ExitOK             = 0
	ExitCLIError       = 1
	ExitMissingDep     = 2
	ExitDownloadError  = 3
	ExitTranscodeError = 4
)

// ExitError wraps an error with a process exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sniplette [urls...]",
		Short:         "Tiny video helper for snack-sized clips",
		Long:          "Sniplette is a tiny video helper that turns large Instagram and YouTube videos into small, shareable clips. Give it a link, and Sniplette will fetch → transcode → compress → and hand you a neat little 'snip' perfect for messaging apps, chats, and social platforms.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1), // preserve current behavior: requires at least one URL
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to the same behavior as the old CLI when no subcommand is specified.
			return runExecute(cmd, args, runMode{
				ForceTUI:   false,
				DryRunOnly: false,
			})
		},
	}

	// Persistent flags available to all subcommands
	root.PersistentFlags().StringP("out-dir", "o", ".", "Output directory")
	root.PersistentFlags().BoolP("verbose", "v", false, "Show full subprocess commands/output")
	root.PersistentFlags().String("dl-binary", "", "Path to yt-dlp or youtube-dl")
	root.PersistentFlags().Int("jobs", 2, "Max concurrent jobs in TUI")

	// Also bind run-specific flags on root, so `sniplette <url>` continues to work.
	bindRunFlags(root.Flags())

	// Mark compatibility flags as deprecated but functional.
	_ = root.Flags().MarkDeprecated("dry-run", "use 'sniplette plan' instead")
	_ = root.Flags().MarkDeprecated("no-ui", "use 'sniplette tui' or keep using '--no-ui'")

	// Subcommands
	root.AddCommand(newRunCmd())
	root.AddCommand(newPlanCmd())
	root.AddCommand(newTuiCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newCompletionCmd())

	return root
}

func bindRunFlags(fs *pflag.FlagSet) {
	fs.Int("max-size-mb", 50, "Target max size per video (MB). Set 0 to use CRF mode.")
	fs.String("quality-preset", "medium", "Quality preset: low, medium, high")
	fs.Int("resolution", 0, "Override long-side resolution in px (e.g., 540, 720, 1080); 0 uses preset default")
	fs.Bool("audio-only", false, "Extract audio only (M4A)")
	fs.String("caption", "txt", "Caption output: txt, none")
	fs.Bool("keep-temp", false, "Keep intermediate downloads")
	fs.Bool("dry-run", false, "Show plan without executing") // deprecated in favor of 'plan'
	fs.Bool("no-ui", false, "Disable TUI; use plain textual output")
}

// Execute runs the CLI with the provided context.
func Execute(ctx context.Context) error {
	root := newRootCmd()
	return root.ExecuteContext(ctx)
}

// Helpers
func getPersistentString(cmd *cobra.Command, name, def string) string {
	v, err := cmd.InheritedFlags().GetString(name)
	if err != nil || v == "" {
		return def
	}
	return v
}

func ensureDir(path string) error {
	if path == "" {
		path = "."
	}
	return os.MkdirAll(filepath.Clean(path), 0o755)
}