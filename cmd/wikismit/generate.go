package main

import (
	"context"
	"fmt"

	"github.com/scalaview/wikismit/internal/agent"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/pipeline"
	"github.com/spf13/cobra"
)

var agentClientFactory = func() llm.Client {
	return nil
}

var runFullGenerate = pipeline.RunFullGenerate

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Run full documentation generation",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			client := agentClientFactory()
			if client == nil {
				var err error
				client, err = llm.NewClient(cfg.LLM)
				if err != nil {
					return err
				}
			}
			return runGenerate(cmd, cfg, client)
		}),
	}
}

func runGenerate(cmd *cobra.Command, cfg *configpkg.Config, client llm.Client) error {
	err := runFullGenerate(context.Background(), cfg, client)
	if phase4Err, ok := err.(*agent.Phase4Error); ok {
		fmt.Fprintln(cmd.ErrOrStderr(), phase4Err.Summary())
	}
	return err
}
