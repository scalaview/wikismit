package analyzer

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func RunPhase1FileIndex(cfg *configpkg.Config) (store.FileIndex, error) {
	analyzer := NewAnalyzer(cfg.Analysis)
	return analyzer.Analyze(cfg.RepoPath)
}

func RunPhase1(cfg *configpkg.Config) error {
	fileIndex, err := RunPhase1FileIndex(cfg)
	if err != nil {
		return err
	}

	depGraph := BuildDepGraph(fileIndex)
	if err := store.WriteFileIndex(cfg.ArtifactsDir, fileIndex); err != nil {
		return err
	}
	if err := store.WriteDepGraph(cfg.ArtifactsDir, depGraph); err != nil {
		return err
	}

	return nil
}
