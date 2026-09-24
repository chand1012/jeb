package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the jeb server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			url := fmt.Sprintf("http://%s/v1", conf.Address())
			fmt.Fprintln(cmd.OutOrStdout(), "I'm serving on", url, "model", conf.OpenAI.Model)
			// Server implementation will be added separately.
			fmt.Fprintln(cmd.OutOrStdout(), "Server is running")
			return nil
		},
	}
	cmd.Flags().StringP("host", "H", "0.0.0.0", "which network interface are we running on")
	cmd.Flags().IntP("port", "p", 6102, "which port are we running on")
	return cmd
}
