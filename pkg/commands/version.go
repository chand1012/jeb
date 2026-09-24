package commands

import (
	"fmt"

	"github.com/chand1012/jeb/pkg/version"
	"github.com/spf13/cobra"
)

func versionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show build version info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, version.GetVersionInfo())

			check, _ := cmd.Flags().GetBool("check")
			if !check {
				return nil
			}

			updateAvailable, latest, err := version.CheckVersion()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "update check failed: %v\n", err)
				return nil
			}
			if updateAvailable {
				fmt.Fprintf(out, "Update available: %s (current)\n", latest)
			} else {
				fmt.Fprintln(out, "You are running the latest version")
			}
			return nil
		},
	}
	cmd.Flags().Bool("check", false, "check GitHub for a newer release")
	return cmd
}
