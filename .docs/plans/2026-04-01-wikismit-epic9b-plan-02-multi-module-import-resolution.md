# Epic 9B.2 — Multi-Module Import Resolution

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `resolveImports` to iterate over all workspace module paths when matching imports, and update `resolveInternalImportPath` to resolve paths relative to the correct sub-module directory instead of always using `repoPath`.

**Architecture:**

- Change `resolveImports` to check each import against all `workspaceModules` keys (not just `a.modulePath`). When a match is found, record which module path matched so `resolveInternalImportPath` gets the right base directory.
- Change `resolveInternalImportPath` to accept the sub-module's directory as the base for file resolution. For single-module projects, this is still `repoPath`. For workspace projects, it's `filepath.Join(repoPath, subDir)`.
- The `ResolveImportPaths` exported function must also pass the correct module context through.

**Prerequisite:** S9B.1 must be complete (workspace detection + `Analyzer.workspaceModules` field).

---

### Task 1: Add cross-module import resolution tests (RED phase)

**Files:**
- Modify: `internal/analyzer/dep_graph_test.go`

- [ ] **Step 1: Add test for cross-module import being marked internal**

```go
func TestResolveImportsMarksCrossModuleImportAsInternal(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	entry := &store.FileEntry{
		Imports: []store.Import{
			{Path: "github.com/org/shared/pkg/utils"},
		},
	}

	if err := analyzer.resolveImports(filepath.Join(repoPath, "service-a"), entry); err != nil {
		t.Fatalf("resolveImports() error = %v", err)
	}

	if !entry.Imports[0].Internal {
		t.Fatalf("import %q should be marked internal (cross-module workspace)", entry.Imports[0].Path)
	}
}
```

- [ ] **Step 2: Add test for cross-module import resolving to correct file path**

```go
func TestResolveImportsResolvesCrossModuleImportPath(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	entry := &store.FileEntry{
		Imports: []store.Import{
			{Path: "github.com/org/shared/pkg/utils"},
		},
	}

	if err := analyzer.resolveImports(filepath.Join(repoPath, "service-a"), entry); err != nil {
		t.Fatalf("resolveImports() error = %v", err)
	}

	want := "shared/pkg/utils/utils.go"
	if entry.Imports[0].ResolvedPath != want {
		t.Fatalf("ResolvedPath = %q, want %q", entry.Imports[0].ResolvedPath, want)
	}
}
```

- [ ] **Step 3: Add test for external import remaining external in workspace context**

```go
func TestResolveImportsSkipsExternalImportsInWorkspace(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	entry := &store.FileEntry{
		Imports: []store.Import{
			{Path: "fmt"},
			{Path: "github.com/external/something"},
		},
	}

	if err := analyzer.resolveImports(filepath.Join(repoPath, "service-a"), entry); err != nil {
		t.Fatalf("resolveImports() error = %v", err)
	}

	for _, imp := range entry.Imports {
		if imp.Internal {
			t.Fatalf("import %q should remain external", imp.Path)
		}
		if imp.ResolvedPath != "" {
			t.Fatalf("import %q should have empty resolved path", imp.Path)
		}
	}
}
```

- [ ] **Step 4: Run tests to confirm RED**

Run:
```bash
go test ./internal/analyzer -run 'TestResolveImports(CrossModule|SkipsExternal)' -v
```

Expected: FAIL — current `resolveImports` only checks `a.modulePath` prefix, not workspace modules.

---

### Task 2: Refactor `resolveImports` for multi-module

**Files:**
- Modify: `internal/analyzer/dep_graph.go`

- [ ] **Step 1: Refactor `resolveImports` to iterate over workspace modules**

Replace the current single-prefix check with logic that checks all workspace modules:

```go
func (a *Analyzer) resolveImports(repoPath string, entry *store.FileEntry) error {
	for idx := range entry.Imports {
		imp := &entry.Imports[idx]

		matchedModulePath, matchedDir := a.findModuleForImport(imp.Path)
		if matchedModulePath == "" {
			continue
		}

		moduleDir := filepath.Join(repoPath, matchedDir)
		resolvedPath, err := resolveInternalImportPath(moduleDir, matchedModulePath, imp.Path)
		if err != nil {
			return err
		}

		// Make resolved path relative to the workspace root, not the sub-module.
		if matchedDir != "" {
			relFromRoot := filepath.Join(matchedDir, resolvedPath)
			resolvedPath = filepath.ToSlash(relFromRoot)
		}

		imp.Internal = true
		imp.ResolvedPath = resolvedPath
	}

	return nil
}

// findModuleForImport returns the module path and relative directory for the
// import's matching workspace module. Returns ("", "") if no match.
func (a *Analyzer) findModuleForImport(importPath string) (string, string) {
	// Check workspace modules first
	for modPath, dir := range a.workspaceModules {
		if strings.HasPrefix(importPath, modPath) {
			return modPath, dir
		}
	}

	// Fall back to single module path
	if a.modulePath != "" && strings.HasPrefix(importPath, a.modulePath) {
		return a.modulePath, ""
	}

	return "", ""
}
```

Key design decisions:
- `findModuleForImport` is a new helper that encapsulates the "which module does this import belong to?" logic.
- For workspace modules, `resolveInternalImportPath` receives the sub-module directory as `repoPath`, so it resolves correctly within that sub-module.
- The resolved path is then adjusted to be relative to the workspace root by prepending the sub-module directory.
- For single-module projects, `matchedDir` is `""`, so behavior is identical to before.

- [ ] **Step 2: Run the previously RED tests to confirm GREEN**

Run:
```bash
go test ./internal/analyzer -run 'TestResolveImports(CrossModule|SkipsExternal)' -v
```

Expected: ALL PASS.

- [ ] **Step 3: Run existing import resolution tests to confirm no regression**

Run:
```bash
go test ./internal/analyzer -run 'TestResolveInternal|TestBuildDepGraph' -v
```

Expected: ALL PASS — single-module project behavior unchanged.

- [ ] **Step 4: Commit the import resolution refactoring**

```bash
git add internal/analyzer/dep_graph.go internal/analyzer/dep_graph_test.go
git commit -m "feat(analyzer): resolve cross-module imports in workspace (S9B.2)"
```

---

### Task 3: Update `ResolveImportPaths` exported function for workspace

**Files:**
- Modify: `internal/analyzer/dep_graph.go`

- [ ] **Step 1: Refactor `ResolveImportPaths` to support workspace**

The current implementation creates a new `Analyzer` and calls `ensureModulePath` then `resolveImports`. Since `ensureModulePath` already detects workspace vs single-module (from S9B.1), and `resolveImports` now iterates workspace modules (from Task 2 above), the exported function should work as-is. But we need to verify this and ensure `repoPath` is passed correctly.

Current code:
```go
func ResolveImportPaths(repoPath string, cfg configpkg.AnalysisConfig, idx store.FileIndex) (store.FileIndex, error) {
	analyzer := NewAnalyzer(cfg)
	if err := analyzer.ensureModulePath(repoPath); err != nil {
		return nil, err
	}

	resolved := make(store.FileIndex, len(idx))
	for path, entry := range idx {
		entryCopy := entry
		entryCopy.Imports = append([]store.Import(nil), entry.Imports...)
		if err := analyzer.resolveImports(repoPath, &entryCopy); err != nil {
			return nil, err
		}
		resolved[path] = entryCopy
	}

	return resolved, nil
}
```

This should work for workspace because `ensureModulePath` detects workspace modules, and `resolveImports` iterates them. The `repoPath` passed to `resolveImports` is the workspace root, which is correct — `findModuleForImport` determines the sub-module directory.

No code changes needed. Add a test instead:

```go
func TestResolveImportPathsHandlesWorkspaceFixture(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	resolved, err := ResolveImportPaths(repoPath, configpkg.AnalysisConfig{}, idx)
	if err != nil {
		t.Fatalf("ResolveImportPaths() error = %v", err)
	}

	handlerEntry, ok := resolved["service-a/internal/handler/handler.go"]
	if !ok {
		t.Fatal("resolved FileIndex missing service-a/internal/handler/handler.go")
	}

	found := false
	for _, imp := range handlerEntry.Imports {
		if imp.Path == "github.com/org/shared/pkg/utils" {
			found = true
			if !imp.Internal {
				t.Fatal("cross-module import should be marked internal")
			}
			if imp.ResolvedPath != "shared/pkg/utils/utils.go" {
				t.Fatalf("ResolvedPath = %q, want %q", imp.ResolvedPath, "shared/pkg/utils/utils.go")
			}
		}
	}
	if !found {
		t.Fatal("handler missing cross-module import to shared/pkg/utils")
	}
}
```

- [ ] **Step 2: Run the test**

```bash
go test ./internal/analyzer -run 'TestResolveImportPathsHandlesWorkspace' -v
```

Expected: PASS (may fail until S9B.3 workspace file walking is complete, since `Analyze` doesn't walk workspace sub-modules yet). If it fails because `Analyze` doesn't find workspace files, that's expected — mark this test as pending S9B.3 and proceed.

- [ ] **Step 3: Commit test**

```bash
git add internal/analyzer/dep_graph_test.go
git commit -m "test(analyzer): add ResolveImportPaths workspace test (S9B.2)"
```

---

### Task 4: Verify analyzer package regression

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run full analyzer tests**

Run:
```bash
go test ./internal/analyzer -v
```

Expected: ALL PASS.

- [ ] **Step 2: Run full repository regression**

Run:
```bash
go test ./...
```

Expected: ALL PASS, zero failures.
