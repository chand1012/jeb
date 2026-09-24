package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/chand1012/jeb/pkg/config"
	"github.com/chand1012/jeb/pkg/process"
	"github.com/chand1012/jeb/pkg/types"
	"github.com/spf13/cobra"
)

func runRequest(cmd *cobra.Command, _ []string) error {
	conf, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	return executeRequest(cmd.InOrStdin(), cmd.OutOrStdout(), conf)
}

func executeRequest(input io.Reader, output io.Writer, conf *config.Config) error {
	decoder := json.NewDecoder(input)
	var request types.JebRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected one JSON request")
		}
		return fmt.Errorf("decode trailing input: %w", err)
	}

	response, err := process.Request(request, conf)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(response)
}
