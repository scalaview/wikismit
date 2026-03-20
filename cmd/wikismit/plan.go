package main

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	return newStubCmd("plan", "Run planning phase", func(cmd *cobra.Command, cfg *configpkg.Config) error {
		_ = cfg
		return writeCommandOutput(cmd, "not implemented\n")
	})
}
