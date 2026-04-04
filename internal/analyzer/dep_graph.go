package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"golang.org/x/mod/modfile"

	"github.com/scalaview/wikismit/pkg/store"
)

func readModulePath(repoPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}

	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("parsing go.mod: %w", err)
	}
	if file.Module == nil {
		return "", fmt.Errorf("parsing go.mod: missing module declaration")
	}

	return file.Module.Mod.Path, nil
}

func readWorkspaceModules(repoPath string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, "go.work"))
	if err != nil {
		return nil, fmt.Errorf("reading go.work: %w", err)
	}

	workFile, err := modfile.ParseWork("go.work", data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.work: %w", err)
	}

	modules := make(map[string]string)
	for _, use := range workFile.Use {
		subDir := strings.TrimPrefix(use.Path, "./")
		subModPath := filepath.Join(repoPath, subDir)
		modulePath, modErr := readModulePath(subModPath)
		if modErr != nil {
			return nil, fmt.Errorf("reading module in %s: %w", subDir, modErr)
		}
		modules[modulePath] = subDir
	}

	if len(modules) == 0 {
		return nil, fmt.Errorf("go.work has no use directives")
	}

	return modules, nil
}

func (a *Analyzer) ensureModulePath(repoPath string) error {
	if a.modulePath != "" || len(a.workspaceModules) > 0 {
		return nil
	}

	modules, err := readWorkspaceModules(repoPath)
	if err == nil {
		a.workspaceModules = modules
		return nil
	}

	modulePath, err := readModulePath(repoPath)
	if err != nil {
		return err
	}
	a.modulePath = modulePath
	return nil
}

func (a *Analyzer) resolveImports(repoPath string, entry *store.FileEntry) error {
	for idx := range entry.Imports {
		imp := entry.Imports[idx]

		matchedModulePath, matchedDir := a.findModuleForImport(imp.Path)
		if matchedModulePath == "" {
			continue
		}

		moduleDir := filepath.Join(repoPath, matchedDir)
		resolvedPath, err := resolveInternalImportPath(moduleDir, matchedModulePath, imp.Path)
		if err != nil {
			return err
		}

		if matchedDir != "" {
			resolvedPath = filepath.ToSlash(filepath.Join(matchedDir, resolvedPath))
		}

		imp.Internal = true
		imp.ResolvedPath = resolvedPath
	}

	return nil
}

func (a *Analyzer) findModuleForImport(importPath string) (string, string) {
	longestModulePath := ""
	longestModuleDir := ""
	for modPath, dir := range a.workspaceModules {
		if hasModulePathPrefix(importPath, modPath) && len(modPath) > len(longestModulePath) {
			longestModulePath = modPath
			longestModuleDir = dir
		}
	}
	if longestModulePath != "" {
		return longestModulePath, longestModuleDir
	}

	if a.modulePath != "" && hasModulePathPrefix(importPath, a.modulePath) {
		return a.modulePath, ""
	}

	return "", ""
}

func hasModulePathPrefix(importPath string, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func resolveInternalImportPath(repoPath string, modulePath string, importPath string) (string, error) {
	relImportPath := strings.TrimPrefix(importPath, modulePath)
	relImportPath = strings.TrimPrefix(relImportPath, "/")
	dirCandidate := repoPath
	if relImportPath != "" {
		dirCandidate = filepath.Join(repoPath, relImportPath)
	}

	fileCandidate := dirCandidate + ".go"
	if info, err := os.Stat(fileCandidate); err == nil && !info.IsDir() {
		relPath, relErr := filepath.Rel(repoPath, fileCandidate)
		if relErr != nil {
			return "", relErr
		}
		return filepath.ToSlash(relPath), nil
	}

	entries, err := os.ReadDir(dirCandidate)
	if err != nil {
		return "", fmt.Errorf("resolve internal import %q: %w", importPath, err)
	}

	goFiles := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			goFiles = append(goFiles, entry.Name())
		}
	}
	if len(goFiles) == 0 {
		return "", fmt.Errorf("resolve internal import %q: no Go files found", importPath)
	}
	sort.Strings(goFiles)

	resolvedFile := filepath.Join(dirCandidate, goFiles[0])
	relPath, relErr := filepath.Rel(repoPath, resolvedFile)
	if relErr != nil {
		return "", relErr
	}
	return filepath.ToSlash(relPath), nil
}

func buildDepGraph(idx store.FileIndex) store.DepGraph {
	graph := store.DepGraph{}

	filePaths := make([]string, 0, len(idx))
	for filePath := range idx {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	for _, filePath := range filePaths {
		entry := idx[filePath]
		edges := make([]string, 0, len(entry.Imports))
		for _, imp := range entry.Imports {
			if !imp.Internal || imp.ResolvedPath == "" {
				continue
			}
			edges = append(edges, imp.ResolvedPath)
		}
		sort.Strings(edges)
		graph[filePath] = edges
	}

	return graph
}

func BuildDepGraph(idx store.FileIndex) store.DepGraph {
	return buildDepGraph(idx)
}

type DepGraphBuilder struct {
	idx store.FileIndex
}

func NewDepGraphBuilder(idx store.FileIndex) *DepGraphBuilder {
	return &DepGraphBuilder{idx: idx}
}

func (b *DepGraphBuilder) Build() store.DepGraph {
	return buildDepGraph(b.idx)
}

func ResolveImportPaths(repoPath string, cfg *configpkg.Config, idx store.FileIndex) (store.FileIndex, error) {
	analyzer := NewAnalyzer(cfg)
	if err := analyzer.ensureModulePath(repoPath); err != nil {
		return nil, err
	}

	resolved := make(store.FileIndex, len(idx))
	for path, entry := range idx {
		entryCopy := entry
		entryCopy.Imports = append([]*store.Import(nil), entry.Imports...)
		if err := analyzer.resolveImports(repoPath, entryCopy); err != nil {
			return nil, err
		}
		resolved[path] = entryCopy
	}

	return resolved, nil
}
