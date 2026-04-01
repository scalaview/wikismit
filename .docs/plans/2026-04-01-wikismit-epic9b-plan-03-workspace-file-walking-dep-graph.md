# Epic 9B.3 — Workspace-Aware File Walking and Dep Graph

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `Analyzer.Analyze` to walk all workspace sub-module directories when `go.work` is detected, merge them into a single `FileIndex` with correct relative paths, and produce a `DepGraph` that shows cross-module file dependencies.

**Architecture:**

- When workspace is detected, `Analyze` walks each sub-module directory instead of just `repoPath`.
- File paths in the `FileIndex` are prefixed with the sub-module's relative directory (e.g., `service-a/cmd/main.go`).
- `resolveImports` (already refactored in S9B.2) correctly resolves cross-module imports.
- Single-module projects continue to produce the exact same `FileIndex` as before.

**Prerequisite:** S9B.1 (workspace detection) and S9B.2 (multi-module import resolution) must be complete.

---

### Task 1: Add workspace file walking tests (RED phase)

**Files:**
- Modify: `internal/analyzer/analyzer_test.go`

- [ ] **Step 1: Add test for workspace FileIndex containing files from all sub-modules**

```go
func TestAnalyzeIndexesAllWorkspaceSubModules(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	wantFiles := []string{
		"service-a/cmd/main.go",
		"service-a/internal/handler/handler.go",
		"shared/pkg/utils/utils.go",
	}

	for _, path := range wantFiles {
		if _, ok := idx[path]; !ok {
			t.Fatalf("FileIndex missing %q; got keys: %v", path, mapKeys(idx))
		}
	}

	if len(idx) != len(wantFiles) {
		t.Fatalf("len(FileIndex) = %d, want %d; got keys: %v", len(idx), len(wantFiles), mapKeys(idx))
	}
}

func mapKeys(m map[string]store.FileEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

Note: `mapKeys` may already exist or need to be added. Use `sort` import.

- [ ] **Step 2: Add test for workspace DepGraph containing cross-module edges**

```go
func TestAnalyzeWorkspaceDepGraphContainsCrossModuleEdges(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := BuildDepGraph(idx)

	handlerDeps, ok := graph["service-a/internal/handler/handler.go"]
	if !ok {
		t.Fatal("dep graph missing service-a/internal/handler/handler.go")
	}

	found := false
	for _, dep := range handlerDeps {
		if dep == "shared/pkg/utils/utils.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("handler deps = %v, want edge to shared/pkg/utils/utils.go", handlerDeps)
	}
}
```

- [ ] **Step 3: Add test verifying single-module project unchanged**

```go
func TestAnalyzeSingleModuleProjectUnchangedAfterWorkspaceSupport(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	wantFiles := []string{
		"cmd/main.go",
		"internal/api/handler.go",
		"internal/auth/jwt.go",
		"internal/auth/middleware.go",
		"internal/db/client.go",
		"pkg/errors/errors.go",
		"pkg/logger/logger.go",
	}

	if len(idx) != len(wantFiles) {
		t.Fatalf("len(FileIndex) = %d, want %d", len(idx), len(wantFiles))
	}
	for _, path := range wantFiles {
		if _, ok := idx[path]; !ok {
			t.Fatalf("FileIndex missing %q", path)
		}
	}
}
```

- [ ] **Step 4: Run tests to confirm RED**

Run:
```bash
go test ./internal/analyzer -run 'TestAnalyze(Workspace|SingleModule)' -v
```

Expected: FAIL — `Analyze` currently does a single `filepath.WalkDir(repoPath)` and only finds files in the workspace root, not the sub-modules.

---

### Task 2: Refactor `Analyze` for workspace file walking

**Files:**
- Modify: `internal/analyzer/analyzer.go`

- [ ] **Step 1: Add helper to walk a single directory and populate FileIndex**

Extract the walking logic into a helper that accepts a base directory and a path prefix:

```go
// walkDir populates idx with files found under absBase, with keys prefixed by relPrefix.
func (a *Analyzer) walkDir(absBase string, relPrefix string, idx store.FileIndex) error {
	return filepath.WalkDir(absBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, relErr := filepath.Rel(absBase, path)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)

		// Prepend the module prefix for workspace-aware keys
		if relPrefix != "" {
			relPath = relPrefix + "/" + relPath
		}

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

		// For import resolution, pass the workspace root (or repoPath for single-module)
		idx[relPath] = entry
		return nil
	})
}
```

- [ ] **Step 2: Refactor `Analyze` to detect workspace and walk sub-modules**

```go
func (a *Analyzer) Analyze(repoPath string) (store.FileIndex, error) {
	idx := store.FileIndex{}
	if err := a.ensureModulePath(repoPath); err != nil {
		return nil, err
	}

	if len(a.workspaceModules) > 0 {
		// Workspace: walk each sub-module directory
		for _, relDir := range a.workspaceModules {
			subDir := filepath.Join(repoPath, relDir)
			if err := a.walkDir(subDir, relDir, idx); err != nil {
				return nil, err
			}
		}
	} else {
		// Single module: walk repo root
		if err := a.walkDir(repoPath, "", idx); err != nil {
			return nil, err
		}
	}

	// Resolve imports for all collected files
	for filePath, entry := range idx {
		entryCopy := entry
		entryCopy.Imports = append([]store.Import(nil), entry.Imports...)
		if err := a.resolveImports(repoPath, &entryCopy); err != nil {
			return nil, err
		}
		idx[filePath] = entryCopy
	}

	return idx, nil
}
```

Key changes:
- The walking logic is extracted into `walkDir` which handles path prefixing.
- For workspace projects, each sub-module is walked separately with its directory as prefix.
- Import resolution is done after all files are collected, so cross-module imports can be resolved.
- `resolveImports` receives `repoPath` (workspace root) which is correct since `findModuleForImport` (from S9B.2) determines the sub-module directory.

- [ ] **Step 3: Run the previously RED tests to confirm GREEN**

Run:
```bash
go test ./internal/analyzer -run 'TestAnalyze(Workspace|SingleModule)' -v
```

Expected: ALL PASS.

- [ ] **Step 4: Run the cross-module DepGraph test**

Run:
```bash
go test ./internal/analyzer -run 'TestAnalyzeWorkspaceDepGraphContainsCrossModuleEdges' -v
```

Expected: PASS.

- [ ] **Step 5: Run ALL existing analyzer tests to confirm no regression**

Run:
```bash
go test ./internal/analyzer -v
```

Expected: ALL PASS — single-module tests unchanged, workspace tests passing.

- [ ] **Step 6: Commit workspace file walking**

```bash
git add internal/analyzer/analyzer.go internal/analyzer/analyzer_test.go
git commit -m "feat(analyzer): walk workspace sub-modules and build cross-module dep graph (S9B.3)"
```

---

### Task 3: Verify S9B.2 `ResolveImportPaths` workspace test now passes

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run the S9B.2 workspace import resolution test**

Run:
```bash
go test ./internal/analyzer -run 'TestResolveImportPathsHandlesWorkspace' -v
```

Expected: PASS — now that `Analyze` walks workspace sub-modules, the test has files to resolve.

---

### Task 4: Add integration test for `RunPhase1` with workspace

**Files:**
- Modify: `internal/analyzer/analyzer_test.go`

- [ ] **Step 1: Add test for `RunPhase1` with workspace fixture**

```go
func TestRunPhase1HandlesWorkspaceRepo(t *testing.T) {
	artifactsDir := t.TempDir()
	cfg := &configpkg.Config{
		RepoPath:     filepath.Join("..", "..", "testdata", "workspace_repo"),
		ArtifactsDir: artifactsDir,
		Analysis:     configpkg.AnalysisConfig{},
	}

	if err := RunPhase1(cfg); err != nil {
		t.Fatalf("RunPhase1() error = %v", err)
	}

	fileIndexData, err := os.ReadFile(filepath.Join(artifactsDir, "file_index.json"))
	if err != nil {
		t.Fatalf("ReadFile(file_index.json) error = %v", err)
	}
	if len(fileIndexData) == 0 {
		t.Fatal("file_index.json is empty")
	}

	depGraphData, err := os.ReadFile(filepath.Join(artifactsDir, "dep_graph.json"))
	if err != nil {
		t.Fatalf("ReadFile(dep_graph.json) error = %v", err)
	}
	if len(depGraphData) == 0 {
		t.Fatal("dep_graph.json is empty")
	}

	// Verify cross-module edge exists in dep graph JSON
	if !bytes.Contains(depGraphData, []byte("shared/pkg/utils/utils.go")) {
		t.Fatal("dep_graph.json missing cross-module reference to shared/pkg/utils/utils.go")
	}
}
```

- [ ] **Step 2: Run the test**

```bash
go test ./internal/analyzer -run 'TestRunPhase1HandlesWorkspace' -v
```

Expected: PASS.

- [ ] **Step 3: Commit integration test**

```bash
git add internal/analyzer/analyzer_test.go
git commit -m "test(analyzer): add RunPhase1 workspace integration test (S9B.3)"
```

---

### Task 5: Full regression and verification

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run full repository regression**

Run:
```bash
go test ./...
```

Expected: ALL PASS, zero failures.

- [ ] **Step 2: Run `lsp_diagnostics` on all changed files**

Verify zero errors on:
- `internal/analyzer/analyzer.go`
- `internal/analyzer/dep_graph.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/dep_graph_test.go`

Expected: Zero errors, zero warnings.
