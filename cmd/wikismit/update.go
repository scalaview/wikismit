package main

import (
	"context"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/pipeline"
	"github.com/scalaview/wikismit/pkg/store"
	"github.com/spf13/cobra"
)

var updateClientFactory = func() llm.Client {
	return nil
}

func newUpdateCmd() *cobra.Command {
	var baseRef string
	var headRef string
	var changedFiles string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Run incremental update",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			client := updateClientFactory()
			if client == nil {
				client = agentClientFactory()
			}
			if client == nil {
				var err error
				client, err = llm.NewClient(cfg.LLM)
				if err != nil {
					return err
				}
			}

			fallbackToFull := false
			if _, err := store.ReadFileIndex(cfg.ArtifactsDir); err != nil {
				if err == store.ErrArtifactNotFound {
					fallbackToFull = true
				} else {
					return err
				}
			}

			err := pipeline.RunIncremental(context.Background(), cfg, client, pipeline.IncrementalOptions{
				BaseRef:      baseRef,
				HeadRef:      headRef,
				ChangedFiles: changedFiles,
			})
			if err != nil {
				return err
			}
			if fallbackToFull {
				return writeCommandOutput(cmd, "Incremental update complete: no existing artifacts found, fell back to full generate\n")
			}

			if changedFiles != "" {
				return writeCommandOutput(cmd, "Incremental update complete: changed-file override processed\n")
			}
			if baseRef == "" && headRef == "" {
				return writeCommandOutput(cmd, "Incremental update complete\n")
			}
			return writeCommandOutput(cmd, "Incremental update complete\n")
		}),
	}

	cmd.Flags().StringVar(&baseRef, "base-ref", "HEAD~1", "Base git ref for diff")
	cmd.Flags().StringVar(&headRef, "head-ref", "HEAD", "Head git ref for diff")
	cmd.Flags().StringVar(&changedFiles, "changed-files", "", "Comma-separated list of changed files")
	return cmd
}
