package analyzer

import (
	"context"
	"fmt"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func RunPhase1FileIndex(cfg *configpkg.Config) (store.FileIndex, error) {
	analyzer := NewAnalyzer(cfg)
	return analyzer.Analyze(cfg.RepoPath)
}

func RunPhase1(cfg *configpkg.Config) error {
	analyzer := NewAnalyzer(cfg)
	fileIndex, err := analyzer.Analyze(cfg.RepoPath)
	if err != nil {
		return err
	}

	callGraph := LinkCalls(fileIndex)
	depGraph := BuildDepGraph(fileIndex)
	if err := store.WriteFileIndex(cfg.ArtifactsDir, fileIndex); err != nil {
		return fmt.Errorf("writing file index: %w", err)
	}
	if err := store.WriteDepGraph(cfg.ArtifactsDir, depGraph); err != nil {
		return fmt.Errorf("writing dependency graph: %w", err)
	}
	if err := store.WriteCallGraph(cfg.ArtifactsDir, callGraph); err != nil {
		return fmt.Errorf("writing call graph: %w", err)
	}
	if err := analyzer.ExecuteFunctionSummary(context.Background(), fileIndex); err != nil {
		return fmt.Errorf("executing function summary: %w", err)
	}
	if err := store.WriteFileIndex(cfg.ArtifactsDir, fileIndex); err != nil {
		return fmt.Errorf("writing file index: %w", err)
	}

	return nil
}
