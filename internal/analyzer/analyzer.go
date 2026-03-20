package analyzer

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

type Analyzer struct {
	registry        map[string]LanguageParser
	excludePatterns []string
	modulePath      string
	skippedFiles    int
}

func NewAnalyzer(cfg configpkg.AnalysisConfig) *Analyzer {
	excludePatterns := append([]string(nil), cfg.ExcludePatterns...)
	return &Analyzer{
		registry:        registry,
		excludePatterns: excludePatterns,
	}
}

func (a *Analyzer) Analyze(repoPath string) (store.FileIndex, error) {
	idx := store.FileIndex{}

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, relErr := filepath.Rel(repoPath, path)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)
		if a.isExcluded(relPath) {
			return nil
		}

		extension := filepath.Ext(path)
		parser, ok := a.registry[extension]
		if !ok {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		entry, parseErr := parser.ExtractSymbols(path, src)
		if parseErr != nil {
			a.skippedFiles++
			return nil
		}

		idx[relPath] = entry
		return nil
	})
	if err != nil {
		return nil, err
	}

	return idx, nil
}

func (a *Analyzer) isExcluded(relPath string) bool {
	for _, pattern := range a.excludePatterns {
		matched, err := doublestar.PathMatch(pattern, relPath)
		if err == nil && matched {
			return true
		}

		matchedBase, baseErr := doublestar.PathMatch(pattern, filepath.Base(relPath))
		if baseErr == nil && matchedBase {
			return true
		}
	}
	return false
}
