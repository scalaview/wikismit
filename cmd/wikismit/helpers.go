package main

import (
	"io"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

func newStubCmd(use string, short string, action func(cmd *cobra.Command, cfg *configpkg.Config) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE:  runWithConfig(action),
	}
}

func writeCommandOutput(cmd *cobra.Command, msg string) error {
	_, err := io.WriteString(cmd.OutOrStdout(), msg)
	return err
}
