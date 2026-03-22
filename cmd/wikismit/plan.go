package main

import (
	"context"
	"fmt"

	"github.com/scalaview/wikismit/internal/analyzer"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/planner"
	"github.com/scalaview/wikismit/pkg/store"
	"github.com/spf13/cobra"
)

var plannerClientFactory = func() llm.Client {
	return nil
}

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Run planning phase",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			if err := analyzer.RunPhase1(cfg); err != nil {
				return err
			}

			idx, err := store.ReadFileIndex(cfg.ArtifactsDir)
			if err != nil {
				return err
			}
			graph, err := store.ReadDepGraph(cfg.ArtifactsDir)
			if err != nil {
				return err
			}

			client := plannerClientFactory()
			if client == nil {
				client, err = llm.NewClient(cfg.LLM)
				if err != nil {
					return err
				}
			}

			navPlan, err := planner.RunPlanner(context.Background(), idx, graph, cfg, client)
			if err != nil {
				return err
			}
			if err := store.WriteNavPlan(cfg.ArtifactsDir, *navPlan); err != nil {
				return err
			}

			return writeCommandOutput(cmd, fmt.Sprintf("nav_plan.json written to %s\n", cfg.ArtifactsDir))
		}),
	}
}
