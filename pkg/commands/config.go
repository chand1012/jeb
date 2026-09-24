package commands

import (
	"fmt"

	"github.com/chand1012/jeb/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var configKeys = map[string]string{
	"host":             "server.host",
	"port":             "server.port",
	"base-url":         "openai.base_url",
	"api-key":          "openai.api_key",
	"model":            "openai.model",
	"max-tokens":       "openai.max_tokens",
	"reasoning-effort": "openai.reasoning_effort",
	"timeout":          "openai.timeout",
	"max-retries":      "openai.max_retries",
	"max-requests":     "concurrency.max_requests",
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	overrides := make(map[string]any)
	var flagErr error
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flagErr != nil {
			return
		}
		key, ok := configKeys[flag.Name]
		if !ok {
			return
		}
		var value any
		switch flag.Value.Type() {
		case "string":
			value, flagErr = cmd.Flags().GetString(flag.Name)
		case "int":
			value, flagErr = cmd.Flags().GetInt(flag.Name)
		case "duration":
			value, flagErr = cmd.Flags().GetDuration(flag.Name)
		default:
			flagErr = fmt.Errorf("unsupported flag type for %s", flag.Name)
		}
		overrides[key] = value
	})
	if flagErr != nil {
		return nil, flagErr
	}
	return config.LoadWithOverrides(overrides)
}
