package main

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

func newBuildCmd() *cobra.Command {
	return newStubCmd("build", "Build VitePress site", func(cmd *cobra.Command, cfg *configpkg.Config) error {
		_ = cfg
		return writeCommandOutput(cmd, "not implemented\n")
	})
}
