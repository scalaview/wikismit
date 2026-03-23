package main

import (
	"context"
	"fmt"

	"github.com/scalaview/wikismit/internal/agent"
	"github.com/scalaview/wikismit/internal/analyzer"
	"github.com/scalaview/wikismit/internal/composer"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
	"github.com/spf13/cobra"
)

var agentClientFactory = func() llm.Client {
	return nil
}

var depGraphReader = store.ReadDepGraph

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
	if err := analyzer.RunPhase1(cfg); err != nil {
		return err
	}

	idx, err := store.ReadFileIndex(cfg.ArtifactsDir)
	if err != nil {
		return err
	}
	graph, err := depGraphReader(cfg.ArtifactsDir)
	if err != nil {
		return err
	}
	plan, err := store.ReadNavPlan(cfg.ArtifactsDir)
	if err != nil {
		return err
	}
	sharedContext, err := store.ReadSharedContext(cfg.ArtifactsDir)
	if err != nil {
		return err
	}

	modules := make([]store.Module, 0, len(plan.Modules))
	for _, module := range plan.Modules {
		if module.Owner == "agent" {
			modules = append(modules, module)
		}
	}

	err = agent.Run(context.Background(), modules, agent.AgentInput{
		FileIndex:     idx,
		SharedContext: sharedContext,
		Config:        cfg,
	}, client, cfg.ArtifactsDir, cfg.Agent.Concurrency)
	if phase4Err, ok := err.(*agent.Phase4Error); ok {
		fmt.Fprintln(cmd.ErrOrStderr(), phase4Err.Summary())
	}
	if err != nil {
		return err
	}

	return composer.RunComposer(cfg, &plan, idx, graph)
}
