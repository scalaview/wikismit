package main

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return newStubCmd("update", "Run incremental update", func(cmd *cobra.Command, cfg *configpkg.Config) error {
		_ = cfg
		return writeCommandOutput(cmd, "not implemented\n")
	})
}
