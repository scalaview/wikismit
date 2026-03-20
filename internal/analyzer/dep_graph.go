package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func (a *Analyzer) ensureModulePath(repoPath string) error {
	if a.modulePath != "" {
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
		imp := &entry.Imports[idx]
		if !strings.HasPrefix(imp.Path, a.modulePath) {
			continue
		}

		resolvedPath, err := resolveInternalImportPath(repoPath, a.modulePath, imp.Path)
		if err != nil {
			return err
		}
		imp.Internal = true
		imp.ResolvedPath = resolvedPath
	}

	return nil
}

func resolveInternalImportPath(repoPath string, modulePath string, importPath string) (string, error) {
	relImportPath := strings.TrimPrefix(importPath, modulePath)
	relImportPath = strings.TrimPrefix(relImportPath, "/")
	if relImportPath == "" {
		return "", fmt.Errorf("resolve internal import %q: empty relative path", importPath)
	}

	fileCandidate := filepath.Join(repoPath, relImportPath+".go")
	if info, err := os.Stat(fileCandidate); err == nil && !info.IsDir() {
		relPath, relErr := filepath.Rel(repoPath, fileCandidate)
		if relErr != nil {
			return "", relErr
		}
		return filepath.ToSlash(relPath), nil
	}

	dirCandidate := filepath.Join(repoPath, relImportPath)
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

func BuildDepGraph(idx store.FileIndex) store.DepGraph {
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
