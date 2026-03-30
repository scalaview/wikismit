package main

import (
	"io"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
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

func resolveClient(factory func() llm.Client, cfg *configpkg.Config) (llm.Client, error) {
	if client := factory(); client != nil {
		return client, nil
	}
	return llm.NewClient(cfg.LLM)
}

func resolveClientWithFallback(primaryFactory, fallbackFactory func() llm.Client, cfg *configpkg.Config) (llm.Client, error) {
	if client := primaryFactory(); client != nil {
		return client, nil
	}
	return resolveClient(fallbackFactory, cfg)
}
