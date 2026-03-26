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

func existingConfigPath(outputDir string) (string, error) {
	for _, name := range []string{"config.mts", "config.ts"} {
		path := filepath.Join(outputDir, ".vitepress", name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", os.ErrNotExist
}

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build VitePress site",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			if _, err := existingConfigPath(cfg.OutputDir); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("missing VitePress config; run wikismit generate first")
				}
				return err
			}
			if _, err := lookPath("node"); err != nil {
				return errors.New("Node.js 20+ is required to build the VitePress site")
			}

			nodeModulesPath := filepath.Join(cfg.OutputDir, "node_modules")
			packageJSONPath := filepath.Join(cfg.OutputDir, "package.json")
			_, packageJSONErr := os.Stat(packageJSONPath)
			if _, err := os.Stat(nodeModulesPath); errors.Is(err, os.ErrNotExist) {
				installArgs := []string{"install"}
				if packageJSONErr != nil {
					installArgs = []string{"install", "-D", "vitepress"}
				}
				if err := runCommand(cfg.OutputDir, "npm", installArgs...); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}

			if packageJSONErr == nil {
				if err := runCommand(cfg.OutputDir, "npm", "run", "docs:build"); err != nil {
					return err
				}
			} else if err := runCommand(cfg.OutputDir, "npx", "vitepress", "build", "docs"); err != nil {
				return err
			}
			return writeCommandOutput(cmd, fmt.Sprintf("Build complete: VitePress site built from %s\n", cfg.OutputDir))
		}),
	}
}
