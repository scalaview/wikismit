package analyzer

import (
	"context"
	"fmt"

	"github.com/scalaview/wikismit/internal/agent"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

type functionSummaryExecutor interface {
	ExecuteFunctionSummary(ctx context.Context, idx store.FileIndex, metrics store.MetricsMap) error
}

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
	BuildCalledByIndex(fileIndex, callGraph)
	depGraph := BuildDepGraph(fileIndex)

	metrics := NewMetricsComputer().Compute(fileIndex, callGraph)
	if err := store.WriteMetrics(cfg.ArtifactsDir, metrics); err != nil {
		return fmt.Errorf("writing function metrics: %w", err)
	}

	if err := store.WriteFileIndex(cfg.ArtifactsDir, fileIndex); err != nil {
		return fmt.Errorf("writing file index: %w", err)
	}
	if err := store.WriteDepGraph(cfg.ArtifactsDir, depGraph); err != nil {
		return fmt.Errorf("writing dependency graph: %w", err)
	}
	if err := store.WriteCallGraph(cfg.ArtifactsDir, callGraph); err != nil {
		return fmt.Errorf("writing call graph: %w", err)
	}
	if err := executeFunctionSummaryAndPersistEventFacts(context.Background(), analyzer, fileIndex, metrics, cfg.ArtifactsDir, nil); err != nil {
		return err
	}

	return nil
}

func executeFunctionSummaryAndPersistEventFacts(ctx context.Context, executor functionSummaryExecutor, fileIndex store.FileIndex, metrics store.MetricsMap, artifactsDir string, indexer *agent.EventIndexer) error {
	if err := executor.ExecuteFunctionSummary(ctx, fileIndex, metrics); err != nil {
		return fmt.Errorf("executing function summary: %w", err)
	}
	if err := store.WriteFileIndex(artifactsDir, fileIndex); err != nil {
		return fmt.Errorf("writing file index: %w", err)
	}
	if indexer == nil {
		indexer = agent.NewEventIndexer("", nil)
	}
	eventFactIndex := indexer.Build(fileIndex)
	if err := store.WriteEventFactIndex(artifactsDir, eventFactIndex); err != nil {
		return fmt.Errorf("writing event fact index: %w", err)
	}
	return nil
}
