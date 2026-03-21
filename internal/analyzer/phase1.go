package analyzer

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func RunPhase1(cfg *configpkg.Config) error {
	analyzer := NewAnalyzer(cfg.Analysis)
	fileIndex, err := analyzer.Analyze(cfg.RepoPath)
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
