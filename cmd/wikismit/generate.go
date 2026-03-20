package main

import (
	"github.com/scalaview/wikismit/internal/analyzer"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Run full documentation generation",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			_ = cmd
			return analyzer.RunPhase1(cfg)
		}),
	}
}
