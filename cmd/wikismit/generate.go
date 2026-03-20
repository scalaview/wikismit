package main

import (
	"io"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Run full documentation generation",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			_, err = io.WriteString(cmd.OutOrStdout(), string(data))
			if err != nil {
				return err
			}
			return nil
		}),
	}
}
