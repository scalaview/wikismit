package main

import (
	"fmt"
	"os"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

var configPath string
var verbose bool
var repoPathOverride string
var outputDirOverride string
var artifactsDirOverride string

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "wikismit",
		Short: "Generate repository wiki documentation",
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "./config.yaml", "Path to config.yaml")
	rootCmd.PersistentFlags().StringVar(&repoPathOverride, "repo", "", "Repository root path")
	rootCmd.PersistentFlags().StringVar(&outputDirOverride, "output", "", "Output directory for generated docs")
	rootCmd.PersistentFlags().StringVar(&artifactsDirOverride, "artifacts", "", "Artifacts directory")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	rootCmd.AddCommand(
		newGenerateCmd(),
		newUpdateCmd(),
		newPlanCmd(),
		newValidateCmd(),
		newBuildCmd(),
	)

	return rootCmd
}

func loadAndValidateConfig() (*configpkg.Config, error) {
	cfg, err := configpkg.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	applyCLIOverrides(cfg)
	return cfg, nil
}

func applyCLIOverrides(cfg *configpkg.Config) {
	cfg.Verbose = verbose
	if repoPathOverride != "" {
		cfg.RepoPath = repoPathOverride
	}
	if outputDirOverride != "" {
		cfg.OutputDir = outputDirOverride
	}
	if artifactsDirOverride != "" {
		cfg.ArtifactsDir = artifactsDirOverride
	}
}

func runWithConfig(action func(cmd *cobra.Command, cfg *configpkg.Config) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := loadAndValidateConfig()
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			return err
		}
		return action(cmd, cfg)
	}
}
