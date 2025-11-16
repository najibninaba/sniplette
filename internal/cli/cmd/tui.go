package cmd

import (
	"github.com/spf13/cobra"
)

func newTuiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "tui [urls...]",
		Short:         "Force TUI mode for interactive snips",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		PreRunE:       runPreRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Force TUI; if stdout is not a terminal, ui.Run will error appropriately.
			return runExecute(cmd, args, runMode{
				ForceTUI:   true,
				DryRunOnly: false,
			})
		},
	}
	bindRunFlags(cmd.Flags())
	// In TUI mode, '--no-ui' makes no sense, but keep flag for compatibility.
	if f := cmd.Flags().Lookup("no-ui"); f != nil {
		f.Hidden = true
	}
	return cmd
}