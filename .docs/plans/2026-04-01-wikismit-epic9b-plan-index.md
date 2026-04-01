# wikismit Epic 9B Plan Index

Use this index instead of executing `.docs/tasks/wikismit-epic9b-go-workspace.md` directly.

## Read first

1. `.docs/tasks/wikismit-epic9b-go-workspace.md`
2. `.docs/spec/code-review-oop-refactoring.md` (§4.1)

## Execution order

1. `.docs/plans/2026-04-01-wikismit-epic9b-plan-01-detect-workspace-modules.md`
2. `.docs/plans/2026-04-01-wikismit-epic9b-plan-02-multi-module-import-resolution.md`
3. `.docs/plans/2026-04-01-wikismit-epic9b-plan-03-workspace-file-walking-dep-graph.md`

## Why this split exists

Epic 9B adds `go.work` multi-module workspace support to the analyzer. The work proceeds in three strictly ordered layers:

1. **Foundation (S9B.1):** Detect `go.work`, parse workspace modules, extend the `Analyzer` struct to hold multiple module paths. Everything else depends on this.
2. **Logic (S9B.2):** Refactor import resolution to iterate over all workspace module paths instead of a single `modulePath`. Depends on the multi-module data from S9B.1.
3. **Integration (S9B.3):** Refactor `Analyze` to walk all sub-module directories and merge into one `FileIndex`. Depends on both S9B.1 (module detection) and S9B.2 (correct import resolution across modules).

S9B.4 is verification-only — no plan slice needed.

## Pre-implementation alignment notes

1. `Analyzer.modulePath` is currently a `string` field. It must be extended to represent multiple modules. The cleanest approach: add a `workspaceModules map[string]string` field (modulePath → relative dir from workspace root). Keep `modulePath` as a convenience accessor for single-module projects.
2. `golang.org/x/mod/modfile` v0.24.0 is already a dependency. The `modfile.ParseWork` function can parse `go.work` files, extracting `Use` directives that list sub-module directories.
3. `resolveImports` currently does `strings.HasPrefix(imp.Path, a.modulePath)` — a single prefix check. For workspaces, it must iterate over all workspace module paths and find the matching one.
4. `resolveInternalImportPath` receives `(repoPath, modulePath, importPath)` and computes the file path relative to `repoPath`. For workspace sub-modules, it needs the sub-module's directory as the base, not the workspace root.
5. `Analyze` does a single `filepath.WalkDir(repoPath, ...)`. For workspaces, it must walk each sub-module directory separately and prefix file paths with the sub-module's relative directory.
6. Existing `testdata/sample_repo` is a single-module project. A new `testdata/workspace_repo` fixture is needed with `go.work` and multiple sub-modules.

## Commit flow

Recommended commit checkpoints:

1. `readWorkspaceModules` function + workspace detection tests + `Analyzer` struct extension
2. `resolveImports` multi-module refactoring + cross-module import resolution tests
3. `Analyze` workspace file walking refactoring + workspace-aware `FileIndex`/`DepGraph` tests
4. final Epic 9B regression: `go test ./...`

## S9B.4 — Integration Verification (no plan slice)

After all three plans are implemented:

- [ ] `go test ./...` passes with zero failures
- [ ] `testdata/workspace_repo` fixture processes end-to-end through `RunPhase1`
- [ ] `testdata/sample_repo` (single-module) continues to produce identical `FileIndex` and `DepGraph` as before
- [ ] `lsp_diagnostics` on all changed files reports zero errors
