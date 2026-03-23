package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/spf13/cobra"
)

var lookPath = exec.LookPath
var runCommand = func(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build VitePress site",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			configPath := filepath.Join(cfg.OutputDir, ".vitepress", "config.ts")
			if _, err := os.Stat(configPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("missing VitePress config; run wikismit generate first")
				}
				return err
			}
			if _, err := lookPath("node"); err != nil {
				return errors.New("Node.js 20+ is required to build the VitePress site")
			}

			nodeModulesPath := filepath.Join(cfg.OutputDir, "node_modules")
			if _, err := os.Stat(nodeModulesPath); errors.Is(err, os.ErrNotExist) {
				if err := runCommand(cfg.OutputDir, "npm", "install", "-D", "vitepress"); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}

			if err := runCommand(cfg.OutputDir, "npx", "vitepress", "build", "docs"); err != nil {
				return err
			}
			return writeCommandOutput(cmd, fmt.Sprintf("Build complete: VitePress site built from %s\n", cfg.OutputDir))
		}),
	}
}
