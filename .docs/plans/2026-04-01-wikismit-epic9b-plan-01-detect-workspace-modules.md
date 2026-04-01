# Epic 9B.1 — Detect `go.work` and Parse Workspace Modules

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the analyzer to detect `go.work` files, parse their `use` directives, and store the resulting module mapping in the `Analyzer` struct, while preserving full backward compatibility for single-module projects.

**Architecture:**

- Add a `readWorkspaceModules(repoPath string) (map[string]string, error)` function that reads `go.work` and returns `modulePath → relativeDir` (e.g., `"github.com/org/service-a" → "service-a"`).
- Add `workspaceModules map[string]string` field to `Analyzer`. When `go.work` is present, this field is populated. When only `go.mod` exists, it remains `nil` and `modulePath` is used as before.
- Refactor `ensureModulePath` to try `go.work` first, then fall back to `go.mod`.
- Add `testdata/workspace_repo` fixture with `go.work` + 2 sub-modules for testing.

**Tech Stack:** Go, `testing`, `golang.org/x/mod/modfile` (already in `go.mod`), `internal/analyzer` package.

---

### Task 1: Create `testdata/workspace_repo` fixture

**Files:**
- Create: `testdata/workspace_repo/go.work`
- Create: `testdata/workspace_repo/service-a/go.mod`
- Create: `testdata/workspace_repo/service-a/cmd/main.go`
- Create: `testdata/workspace_repo/service-a/internal/handler/handler.go`
- Create: `testdata/workspace_repo/shared/go.mod`
- Create: `testdata/workspace_repo/shared/pkg/utils/utils.go`

- [ ] **Step 1: Create workspace fixture directory structure**

```
testdata/workspace_repo/
├── go.work
├── service-a/
│   ├── go.mod
│   ├── cmd/
│   │   └── main.go
│   └── internal/
│       └── handler/
│           └── handler.go
└── shared/
    ├── go.mod
    └── pkg/
        └── utils/
            └── utils.go
```

`go.work`:
```
go 1.25.0

use ./service-a
use ./shared
```

`service-a/go.mod`:
```
module github.com/org/service-a

go 1.25.0
```

`shared/go.mod`:
```
module github.com/org/shared

go 1.25.0
```

`service-a/cmd/main.go`:
```go
package main

import "github.com/org/service-a/internal/handler"

func main() {
	handler.Handle()
}
```

`service-a/internal/handler/handler.go`:
```go
package handler

import "github.com/org/shared/pkg/utils"

func Handle() {
	utils.DoSomething()
}
```

`shared/pkg/utils/utils.go`:
```go
package utils

func DoSomething() {}
```

- [ ] **Step 2: Verify fixture files exist**

Run:
```bash
find testdata/workspace_repo -type f | sort
```

Expected: 6 files listed matching the structure above.

---

### Task 2: Add tests for workspace detection (RED phase)

**Files:**
- Modify: `internal/analyzer/dep_graph_test.go`

- [ ] **Step 1: Add test for `readWorkspaceModules` with `go.work` present**

```go
func TestReadWorkspaceModulesParsesGoWork(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")

	modules, err := readWorkspaceModules(repoPath)
	if err != nil {
		t.Fatalf("readWorkspaceModules() error = %v", err)
	}

	if len(modules) != 2 {
		t.Fatalf("len(modules) = %d, want 2", len(modules))
	}

	if dir, ok := modules["github.com/org/service-a"]; !ok {
		t.Fatal("missing module github.com/org/service-a")
	} else if dir != "service-a" {
		t.Fatalf("service-a dir = %q, want %q", dir, "service-a")
	}

	if dir, ok := modules["github.com/org/shared"]; !ok {
		t.Fatal("missing module github.com/org/shared")
	} else if dir != "shared" {
		t.Fatalf("shared dir = %q, want %q", dir, "shared")
	}
}
```

- [ ] **Step 2: Add test for `readWorkspaceModules` returning error when no `go.work`**

```go
func TestReadWorkspaceModulesReturnsErrorWithoutGoWork(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := readWorkspaceModules(tmpDir)
	if err == nil {
		t.Fatal("readWorkspaceModules() expected error for directory without go.work")
	}
}
```

- [ ] **Step 3: Add test for `readModulePath` still working on single-module project**

This test already exists as `TestReadModulePathReturnsGoModModule`. Verify it still passes:
```bash
go test ./internal/analyzer -run 'TestReadModulePath' -v
```

Expected: PASS (unchanged).

- [ ] **Step 4: Add test for `ensureModulePath` detecting workspace vs single-module**

```go
func TestEnsureModulePathDetectsWorkspace(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	if len(analyzer.workspaceModules) != 2 {
		t.Fatalf("workspaceModules len = %d, want 2", len(analyzer.workspaceModules))
	}
}

func TestEnsureModulePathFallsBackToGoMod(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	if analyzer.modulePath != "github.com/wikismit/sample" {
		t.Fatalf("modulePath = %q, want %q", analyzer.modulePath, "github.com/wikismit/sample")
	}
	if len(analyzer.workspaceModules) != 0 {
		t.Fatalf("workspaceModules len = %d, want 0 for single-module project", len(analyzer.workspaceModules))
	}
}
```

- [ ] **Step 5: Run tests to confirm RED**

Run:
```bash
go test ./internal/analyzer -run 'TestReadWorkspaceModules|TestEnsureModulePath' -v
```

Expected: COMPILE ERROR or FAIL — `readWorkspaceModules` and `workspaceModules` don't exist yet.

---

### Task 3: Implement `readWorkspaceModules` and extend `Analyzer` struct

**Files:**
- Modify: `internal/analyzer/dep_graph.go`
- Modify: `internal/analyzer/analyzer.go`

- [ ] **Step 1: Add `readWorkspaceModules` function to `dep_graph.go`**

Add after `readModulePath`:

```go
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
		subDir := use.Path
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
```

- [ ] **Step 2: Add `workspaceModules` field to `Analyzer` struct in `analyzer.go`**

```go
type Analyzer struct {
	registry          map[string]LanguageParser
	excludePatterns   []string
	modulePath        string
	workspaceModules  map[string]string // modulePath → relative dir from workspace root
	skippedFiles      int
}
```

- [ ] **Step 3: Refactor `ensureModulePath` in `dep_graph.go` to try workspace first**

```go
func (a *Analyzer) ensureModulePath(repoPath string) error {
	if a.modulePath != "" || len(a.workspaceModules) > 0 {
		return nil
	}

	// Try go.work first
	modules, err := readWorkspaceModules(repoPath)
	if err == nil {
		a.workspaceModules = modules
		return nil
	}

	// Fall back to single go.mod
	modulePath, err := readModulePath(repoPath)
	if err != nil {
		return err
	}
	a.modulePath = modulePath
	return nil
}
```

- [ ] **Step 4: Run the previously RED tests to confirm GREEN**

Run:
```bash
go test ./internal/analyzer -run 'TestReadWorkspaceModules|TestEnsureModulePath' -v
```

Expected: ALL PASS.

- [ ] **Step 5: Run existing analyzer tests to confirm no regression**

Run:
```bash
go test ./internal/analyzer -v
```

Expected: ALL PASS.

- [ ] **Step 6: Commit the foundation**

```bash
git add testdata/workspace_repo internal/analyzer/dep_graph.go internal/analyzer/analyzer.go internal/analyzer/dep_graph_test.go
git commit -m "feat(analyzer): detect go.work and parse workspace modules (S9B.1)"
```

---

### Task 4: Add edge-case tests for workspace detection

**Files:**
- Modify: `internal/analyzer/dep_graph_test.go`

- [ ] **Step 1: Add test for directory with neither `go.work` nor `go.mod`**

```go
func TestEnsureModulePathReturnsErrorWithoutGoModOrGoWork(t *testing.T) {
	tmpDir := t.TempDir()
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	err := analyzer.ensureModulePath(tmpDir)
	if err == nil {
		t.Fatal("ensureModulePath() expected error for directory without go.mod or go.work")
	}
}
```

- [ ] **Step 2: Add test for `go.work` with empty `use` directives**

```go
func TestReadWorkspaceModulesRejectsEmptyUseDirectives(t *testing.T) {
	tmpDir := t.TempDir()
	goWorkContent := "go 1.25.0\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.work"), []byte(goWorkContent), 0o644); err != nil {
		t.Fatalf("WriteFile(go.work) error = %v", err)
	}

	_, err := readWorkspaceModules(tmpDir)
	if err == nil {
		t.Fatal("readWorkspaceModules() expected error for go.work with no use directives")
	}
}
```

- [ ] **Step 3: Run all dep_graph tests**

Run:
```bash
go test ./internal/analyzer -run 'TestReadWorkspace|TestEnsureModulePath' -v
```

Expected: ALL PASS.

- [ ] **Step 4: Commit edge-case tests**

```bash
git add internal/analyzer/dep_graph_test.go
git commit -m "test(analyzer): add workspace detection edge cases (S9B.1)"
```

---

### Task 5: Verify analyzer package regression

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run full repository regression**

Run:
```bash
go test ./...
```

Expected: ALL PASS, zero failures.
