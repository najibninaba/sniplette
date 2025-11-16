package cmd

import (
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plan [urls...]",
		Short:         "Show plan (metadata-only) without executing",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		PreRunE:       runPreRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecute(cmd, args, runMode{
				ForceTUI:   false,
				DryRunOnly: true,
			})
		},
	}
	// Reuse same flags; plan ignores actual encode
	bindRunFlags(cmd.Flags())
	return cmd
}