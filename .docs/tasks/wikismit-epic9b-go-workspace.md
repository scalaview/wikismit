# wikismit — Epic 9B: Go Workspace 支持

**Status:** `todo`
**Depends on:** Epic 8C
**Goal:** 扩展 analyzer 支持带有 `go.work` 的多模块 Go 项目，使其能正确解析跨模块内部依赖关系。
**Spec refs:** code-review-oop-refactoring.md §4.1 C

---

## S9B.1 — Detect `go.work` and Parse Workspace Modules

**Status:** `todo`

**Description:**
当前 `readModulePath` 只读取根目录的 `go.mod`，对于 workspace 项目根目录没有 `go.mod` 会直接报错。需要新增 `go.work` 检测逻辑：解析 `go.work` 文件中的 `use` 指令，获取所有子模块目录和各自的 `modulePath`。

**Acceptance criteria:**
- 根目录有 `go.work` 时，解析其中的 `use` 指令获取子模块目录列表
- 根目录只有 `go.mod` 时，保持现有行为不变
- 每个子模块的 `modulePath` 被正确读取
- `Analyzer` 的 `modulePath` 字段能表示多个模块（或被 workspace 映射替代）

**Files to modify:**
```
internal/analyzer/dep_graph.go
internal/analyzer/analyzer.go
internal/analyzer/dep_graph_test.go
```

### Subtasks

#### S9B.1.1 — Add Workspace Detection Test
Legacy - Add tests for `readModulePath` returning single module (existing behavior)
- Add tests with `go.work` fixture returning multiple module paths
- Add tests with neither `go.mod` nor `go.work` returning an error

#### S9B.1.2 — Implement `go.work` Detection and Parsing
Legacy - Add `readWorkspaceModules` function that reads `go.work` and returns `map[string]string` (modulePath → local directory)
- Use `golang.org/x/mod/modfile` which already supports `go.work` parsing via `ParseWorkFile` or manual parsing of `use` directives
- Keep `readModulePath` as a fallback for single-module projects

#### S9B.1.3 — Refactor `Analyzer` to Support Multiple Module Paths
Legacy - Change `Analyzer.modulePath string` to support multiple module paths
- Update `ensureModulePath` to detect workspace vs single module and store the full mapping
- Keep the `Analyzer` struct interface stable for downstream code

#### S9B.1.4 — Verify Workspace Detection
Legacy - Run `go test ./internal/analyzer -v`
- Verify single-module projects still work unchanged

---

## S9B.2 — Multi-Module Import Resolution

**Status:** `todo`

**Description:**
当前 `resolveImports` 只匹配单一 `modulePath` 前缀。对于 workspace 项目，`service-a` 的文件可能 `import "github.com/org/shared/pkg/utils"` — 这是跨模块的内部依赖，需要遍历所有 workspace 模块来查找匹配。

**Acceptance criteria:**
- Cross-module imports (e.g., `service-a` importing from `shared`) are correctly identified as internal
- `resolveInternalImportPath` resolves paths relative to the correct sub-module directory, not always `repoPath`
- Single-module projects continue to work unchanged
- `FileIndex` contains files from all workspace sub-modules

**Files to modify:**
```
internal/analyzer/dep_graph.go
internal/analyzer/analyzer.go
internal/analyzer/dep_graph_test.go
```

### Subtasks

#### S9B.2.1 — Add Cross-Module Import Resolution Test
Legacy - Add tests with a workspace fixture where module A imports from module B
- Assert `Import.Internal` is `true` and `ResolvedPath` points to the correct file in module B
- Test that external imports (non-workspace modules) are still skipped

#### S9B.2.2 — Refactor `resolveImports` for Multi-Module
Legacy - Replace single `strings.HasPrefix(imp.Path, a.modulePath)` with iteration over all workspace module paths
- For each import, find the first matching workspace module prefix
- Mark `Internal = true` only if the import matches a workspace module

#### S9B.2.3 — Refactor `resolveInternalImportPath` for Workspace
Legacy - Accept the owning module's directory as base for path resolution (not always `repoPath`)
- Compute `relImportPath` relative to the correct sub-module's directory
- Keep backward compatibility for single-module projects

#### S9B.2.4 — Verify Cross-Module Resolution
Legacy - Run `go test ./internal/analyzer -v`
- Verify that `DepGraph` edges span across workspace sub-modules

---

## S9B.3 — Workspace-Aware File Walking and Dep Graph

**Status:** `todo`

**Description:**
当前 `Analyzer.Analyze` walks a single directory tree and builds one `FileIndex`. For workspace projects, it needs to walk all sub-module directories (listed in `go.work` `use` directives) and merge them into a single `FileIndex` with correct relative paths.

**Acceptance criteria:**
- `Analyzer.Analyze` walks all workspace sub-module directories when a `go.work` is detected
- `FileIndex` keys use paths relative to the workspace root (e.g., `service-a/internal/handler.go`)
- `DepGraph` correctly shows cross-module file dependencies
- Single-module projects continue to produce the same `FileIndex` as before

**Files to modify:**
```
internal/analyzer/analyzer.go
internal/analyzer/dep_graph.go
internal/analyzer/analyzer_test.go
```

### Subtasks

#### S9B.3.1 — Add Workspace File Walking Test
Legacy - Add integration test with a multi-module workspace fixture
- Assert `FileIndex` contains files from all sub-modules with correct relative paths
- Assert `DepGraph` contains cross-module edges

#### S9B.3.2 — Refactor `Analyze` for Workspace File Walking
Legacy - When workspace is detected, walk each sub-module directory instead of just `repoPath`
- Merge all sub-module `FileIndex` results into one unified index
- Ensure relative paths are computed from the workspace root, not the sub-module root

#### S9B.3.3 — Verify Workspace Dep Graph Integrity
Legacy - Run `go test ./internal/analyzer -v`
- Verify `DepGraph` shows edges like `service-a/internal/handler.go → shared/pkg/utils/utils.go`

---

## S9B.4 — Integration Verification

**Status:** `todo`

**Description:**
Verify that the full pipeline works with a Go workspace project: analysis, planning, and dependency graph all produce correct results.

**Acceptance criteria:**
- `go test ./...` passes with zero failures
- A workspace test fixture can be processed end-to-end through `RunPhase1`
- Single-module test fixtures continue to work unchanged
- No diagnostic errors in changed files

**Files to modify:**
```
(none — verification only, possibly testdata additions)
```
