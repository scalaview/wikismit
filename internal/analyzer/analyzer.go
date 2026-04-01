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
	registry         map[string]LanguageParser
	excludePatterns  []string
	modulePath       string
	workspaceModules map[string]string // modulePath → relative dir from workspace root
	skippedFiles     int
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
	if err := a.ensureModulePath(repoPath); err != nil {
		return nil, err
	}

	if len(a.workspaceModules) > 0 {
		moduleDirs := make([]string, 0, len(a.workspaceModules))
		seenDirs := make(map[string]bool, len(a.workspaceModules))
		for _, moduleDir := range a.workspaceModules {
			if seenDirs[moduleDir] {
				continue
			}
			seenDirs[moduleDir] = true
			moduleDirs = append(moduleDirs, moduleDir)
		}
		for _, moduleDir := range moduleDirs {
			if err := a.walkRepoDir(repoPath, filepath.Join(repoPath, moduleDir), idx); err != nil {
				return nil, err
			}
		}
	} else {
		if err := a.walkRepoDir(repoPath, repoPath, idx); err != nil {
			return nil, err
		}
	}

	for path, entry := range idx {
		entryCopy := entry
		entryCopy.Imports = append([]store.Import(nil), entry.Imports...)
		if err := a.resolveImports(repoPath, &entryCopy); err != nil {
			return nil, err
		}
		idx[path] = entryCopy
	}

	return idx, nil
}

func (a *Analyzer) walkRepoDir(repoPath string, rootPath string, idx store.FileIndex) error {
	return filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
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
