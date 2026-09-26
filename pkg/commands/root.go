package commands

import (
	"time"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "jeb",
		Short:             "Evaluate a JSON request",
		Args:              cobra.NoArgs,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	flags := cmd.PersistentFlags()
	flags.StringP("model", "m", "qwen3.5:9b", "model to send")
	flags.String("base-url", "http://localhost:11434/v1", "OpenAI-compatible API base URL")
	flags.String("api-key", "", "API key")
	flags.Int("max-tokens", 10, "maximum output tokens")
	flags.String("reasoning-effort", "none", "reasoning effort")
	flags.Duration("timeout", 60*time.Second, "request timeout")
	flags.Int("max-retries", 3, "retries after a failed completion (3 means up to 4 attempts)")
	flags.Int("max-requests", 1, "maximum concurrent requests")
	cmd.RunE = runRequest
	cmd.AddCommand(serveCommand(), versionCommand())
	return cmd
}
