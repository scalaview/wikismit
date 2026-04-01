# Epic 9A.1 Fix dependencyDepth Using Wrong Graph for Module Sorting

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `GenerateIndexPage` so that modules are sorted by their actual dependency depth, by replacing the file-level `DepGraph` with a module-level graph built from `NavPlan` fields.

**Architecture:** Add a `buildModuleGraph(plan *store.NavPlan) map[string][]string` function that extracts module-to-module edges from `Module.DependsOnShared` and `Module.ReferencedBy`. Pass the module graph to `dependencyDepth` instead of the file-level `DepGraph`. Keep `GenerateIndexPage` signature unchanged — the `graph store.DepGraph` parameter becomes unused internally but is preserved for API compatibility.

**Tech Stack:** Go, `testing`, existing `internal/composer` package, `pkg/store` types.

---

### Task 1: Add failing test proving current code mis-sorts with file-level graph

**Files:**
- Modify: `internal/composer/renderer_test.go:121-144`
- Test: `internal/composer/renderer_test.go`

- [ ] **Step 1: Add a test that exposes the file-level graph mismatch**

Add `TestGenerateIndexPageSortsModulesUsingModuleGraphNotFileGraph` after the existing `TestGenerateIndexPageListsModulesByDependencyDepth`. This test uses a realistic file-level `DepGraph` (keys = file paths) while module IDs are short names like `"api"`, `"auth"`, `"db"`. The current code will fail to sort because the keys never match.

```go
func TestGenerateIndexPageSortsModulesUsingModuleGraphNotFileGraph(t *testing.T) {
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "api", Shared: false, DependsOnShared: []string{"auth"}},
		{ID: "auth", Shared: true, ReferencedBy: []string{"api"}},
		{ID: "db", Shared: true},
	}}

	// This is a file-level DepGraph — keys are file paths, not module IDs.
	// The old code passes this directly to dependencyDepth, which looks up
	// module IDs as keys. The keys never match, so depth is always 0.
	fileGraph := store.DepGraph{
		"internal/api/server.go":    {"internal/auth/jwt.go", "internal/db/conn.go"},
		"internal/auth/jwt.go":      {"internal/db/conn.go"},
		"internal/db/conn.go":       nil,
	}

	result := GenerateIndexPage(plan, fileGraph)

	dbIndex := strings.Index(result, "| db |")
	authIndex := strings.Index(result, "| auth |")
	apiIndex := strings.Index(result, "| api |")
	if dbIndex == -1 || authIndex == -1 || apiIndex == -1 {
		t.Fatalf("result missing module rows:\n%s", result)
	}
	if !(dbIndex < authIndex && authIndex < apiIndex) {
		t.Fatalf("modules not ordered shallowest-first: db(depth=0) then auth(depth=1) then api(depth=2):\n%s", result)
	}
}
```

- [ ] **Step 2: Run the test to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestGenerateIndexPageSortsModulesUsingModuleGraphNotFileGraph' -v
```

Expected: FAIL — all modules get depth 0 because `"api"` is not a key in the file-level graph, so sorting is arbitrary or alphabetical.

- [ ] **Step 3: Commit the RED test**

```bash
git add internal/composer/renderer_test.go
git commit -m "test: expose dependencyDepth file-level graph mismatch"
```

---

### Task 2: Add `buildModuleGraph` function and wire it into `GenerateIndexPage`

**Files:**
- Modify: `internal/composer/renderer.go:88-134`
- Test: `internal/composer/renderer_test.go`

- [ ] **Step 1: Add the `buildModuleGraph` function**

Add a new function after `dependencyDepth` (after line 134) in `renderer.go`:

```go
// buildModuleGraph constructs a module-level dependency graph from NavPlan fields.
// Keys are module IDs; values are the module IDs they depend on.
func buildModuleGraph(plan *store.NavPlan) map[string][]string {
	graph := make(map[string][]string)
	for _, mod := range plan.Modules {
		deps := make([]string, 0)
		// DependsOnShared lists shared module IDs this module depends on.
		for _, dep := range mod.DependsOnShared {
			deps = append(deps, dep)
		}
		// ReferencedBy lists module IDs that reference this shared module.
		// For the graph, the shared module depends on nothing — but its
		// dependents list it in DependsOnShared. We only need forward edges.
		graph[mod.ID] = deps
	}
	return graph
}
```

- [ ] **Step 2: Refactor `GenerateIndexPage` to use module graph**

Change `GenerateIndexPage` (renderer.go:88-116) to build and use the module graph internally. Keep the function signature unchanged:

```go
func GenerateIndexPage(plan *store.NavPlan, _ store.DepGraph) string {
	moduleGraph := buildModuleGraph(plan)

	modules := append([]store.Module(nil), plan.Modules...)
	sort.Slice(modules, func(i int, j int) bool {
		leftDepth := dependencyDepth(modules[i].ID, moduleGraph, map[string]bool{})
		rightDepth := dependencyDepth(modules[j].ID, moduleGraph, map[string]bool{})
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return modules[i].ID < modules[j].ID
	})

	var builder strings.Builder
	builder.WriteString("# Documentation Index\n\n")
	builder.WriteString("| Module | Type | Used By |\n")
	builder.WriteString("| --- | --- | --- |\n")
	for _, module := range modules {
		moduleType := "module"
		usedBy := "-"
		if module.Shared {
			moduleType = "shared"
			if len(module.ReferencedBy) > 0 {
				usedBy = strings.Join(module.ReferencedBy, ", ")
			}
		}
		builder.WriteString(fmt.Sprintf("| %s | %s | %s |\n", module.ID, moduleType, usedBy))
	}

	return builder.String()
}
```

Key change: the `graph store.DepGraph` parameter is renamed to `_` to signal it's unused. `dependencyDepth` now receives `moduleGraph` (module-level) instead of the file-level `DepGraph`.

- [ ] **Step 3: Run the previously RED test to confirm GREEN**

Run:

```bash
go test ./internal/composer -run 'TestGenerateIndexPageSortsModulesUsingModuleGraphNotFileGraph' -v
```

Expected: PASS — module graph correctly maps `"api"` → `["auth"]`, `"auth"` → `[]`, `"db"` → `[]`, so depths are computed as api=2, auth=1, db=0.

- [ ] **Step 4: Run the existing dependency depth test to confirm no regression**

Run:

```bash
go test ./internal/composer -run 'TestGenerateIndexPageListsModulesByDependencyDepth' -v
```

Expected: PASS — the existing test still passes because it uses module-ID-keyed graphs, which are now the correct graph type.

- [ ] **Step 5: Commit the fix**

```bash
git add internal/composer/renderer.go internal/composer/renderer_test.go
git commit -m "fix: build module-level graph for dependency depth sorting"
```

---

### Task 3: Add edge-case tests for module graph construction

**Files:**
- Modify: `internal/composer/renderer_test.go`
- Test: `internal/composer/renderer_test.go`

- [ ] **Step 1: Add test for modules with no dependencies appearing first**

```go
func TestGenerateIndexPageModulesWithNoDepsAppearBeforeDependents(t *testing.T) {
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "handler", Shared: false, DependsOnShared: []string{"logger", "config"}},
		{ID: "logger", Shared: true, ReferencedBy: []string{"handler"}},
		{ID: "config", Shared: true, ReferencedBy: []string{"handler"}},
	}}

	result := GenerateIndexPage(plan, store.DepGraph{})

	handlerIndex := strings.Index(result, "| handler |")
	loggerIndex := strings.Index(result, "| logger |")
	configIndex := strings.Index(result, "| config |")
	if handlerIndex == -1 || loggerIndex == -1 || configIndex == -1 {
		t.Fatalf("result missing module rows:\n%s", result)
	}
	if handlerIndex < loggerIndex || handlerIndex < configIndex {
		t.Fatalf("handler (depth=1) should appear after logger and config (depth=0):\n%s", result)
	}
}
```

- [ ] **Step 2: Add test for empty module list**

```go
func TestGenerateIndexPageHandlesEmptyModules(t *testing.T) {
	plan := &store.NavPlan{Modules: []store.Module{}}
	result := GenerateIndexPage(plan, store.DepGraph{})

	if !strings.Contains(result, "# Documentation Index") {
		t.Fatalf("result missing header:\n%s", result)
	}
	if strings.Contains(result, "| api |") {
		t.Fatalf("result should have no module rows for empty plan:\n%s", result)
	}
}
```

- [ ] **Step 3: Run all renderer tests**

Run:

```bash
go test ./internal/composer -run 'TestGenerateIndexPage' -v
```

Expected: ALL PASS.

- [ ] **Step 4: Commit edge-case tests**

```bash
git add internal/composer/renderer_test.go
git commit -m "test: add module graph edge cases for sorting"
```

---

### Task 4: Verify composer package regression

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run full composer package tests**

Run:

```bash
go test ./internal/composer -v
```

Expected: ALL PASS.

- [ ] **Step 2: Run full repository regression**

Run:

```bash
go test ./...
```

Expected: ALL PASS, zero failures.
