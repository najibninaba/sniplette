package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"ig2wa/internal/util/deps"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "doctor",
		Short:         "Diagnose external dependencies (yt-dlp/youtube-dl, ffmpeg)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dl, derr := deps.FindDownloader(getPersistentString(cmd, "dl-binary", ""))
			if derr != nil {
				return &ExitError{Code: ExitMissingDep, Err: derr}
			}
			ff, ferr := deps.FindFFmpeg()
			if ferr != nil {
				return &ExitError{Code: ExitMissingDep, Err: ferr}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Downloader: %s\n", dl)
			fmt.Fprintf(cmd.OutOrStdout(), "FFmpeg:    %s\n", ff)
			return nil
		},
	}
}