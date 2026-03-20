package main

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return newStubCmd("validate", "Validate generated docs", func(cmd *cobra.Command, cfg *configpkg.Config) error {
		_ = cfg
		return writeCommandOutput(cmd, "not implemented\n")
	})
}
