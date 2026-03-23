package main

import (
	"fmt"

	"github.com/scalaview/wikismit/internal/composer"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate generated docs",
		RunE: runWithConfig(func(cmd *cobra.Command, cfg *configpkg.Config) error {
			report, err := composer.ValidateDocs(cfg.OutputDir)
			if err != nil {
				return err
			}
			if err := store.WriteValidationReport(cfg.ArtifactsDir, report); err != nil {
				return err
			}
			return writeCommandOutput(cmd, fmt.Sprintf("Validation complete: %d broken links found in %d files\n", len(report.BrokenLinks), report.TotalFiles))
		}),
	}
}
