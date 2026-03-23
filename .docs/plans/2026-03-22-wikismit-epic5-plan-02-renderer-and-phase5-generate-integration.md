# Epic 5 Renderer and Phase 5 Generate Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the deterministic renderer/composer flow that turns `artifacts/module_docs/` plus Phase 1–4 artifacts into final Markdown under `docs/`, then wire that flow into `wikismit generate`.

**Architecture:** First lock the pure Markdown TOC and index-generation helpers. Then add file-copy orchestration for module/shared docs with citation injection, wrap those helpers in a single `RunComposer` entrypoint, and only after that extend `generate` to load the remaining artifact dependency (`dep_graph.json`) and call Phase 5.

**Tech Stack:** Go, standard `testing`, `os`, `path/filepath`, `sort`, existing `pkg/store`, existing `internal/config`, existing Cobra command tests.

---

### Task 1: Add failing TOC tests before any file-copy logic

**Files:**
- Create: `internal/composer/renderer.go`
- Create: `internal/composer/renderer_test.go`

**Step 1: Write the failing TOC tests**

Add:

```go
func TestGenerateTOCInsertsContentsAfterFirstH1(t *testing.T) {}
func TestGenerateTOCSkipsFilesWithoutH2OrH3Headings(t *testing.T) {}
func TestGenerateTOCBuildsGitHubCompatibleAnchors(t *testing.T) {}
```

Cover these behaviors:

- `## Contents` is inserted after the first H1 heading
- files with no H2/H3 headings stay unchanged
- anchors normalize headings like `Key Functions` and `HTTP Handler`

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestGenerateTOCInsertsContentsAfterFirstH1|TestGenerateTOCSkipsFilesWithoutH2OrH3Headings|TestGenerateTOCBuildsGitHubCompatibleAnchors' -v
```

Expected: FAIL because `GenerateTOC` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func GenerateTOC(content string) string
```

Requirements:

- parse H2/H3 headings only
- build the TOC block deterministically
- insert it after the first H1 if present, otherwise at the top

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing copy/render tests before implementing file emission

**Files:**
- Modify: `internal/composer/renderer.go`
- Modify: `internal/composer/renderer_test.go`

**Step 1: Write the failing copy tests**

Add:

```go
func TestCopyModuleDocsWritesModulesAndSharedDocsToSeparateDirectories(t *testing.T) {}
func TestCopyModuleDocsAppliesCitationsAndTOCBeforeWriting(t *testing.T) {}
```

Cover these behaviors:

- non-shared modules land in `docs/modules/`
- shared modules land in `docs/shared/`
- output content includes citation injection and TOC generation

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestCopyModuleDocsWritesModulesAndSharedDocsToSeparateDirectories|TestCopyModuleDocsAppliesCitationsAndTOCBeforeWriting' -v
```

Expected: FAIL because `CopyModuleDocs` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func CopyModuleDocs(artifactsDir string, docsDir string, plan *store.NavPlan, symbolMap map[string]string) error
```

Requirements:

- read `artifacts/module_docs/{moduleID}.md`
- apply `InjectCitations` then `GenerateTOC`
- write to `docs/modules/` or `docs/shared/` based on `module.Shared`

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Add failing index-page tests before Phase 5 orchestration

**Files:**
- Modify: `internal/composer/renderer.go`
- Modify: `internal/composer/renderer_test.go`

**Step 1: Write the failing index tests**

Add:

```go
func TestGenerateIndexPageListsModulesByDependencyDepth(t *testing.T) {}
func TestGenerateIndexPageIncludesSharedUsedByColumn(t *testing.T) {}
```

Cover these behaviors:

- modules are sorted shallowest-first using dependency depth derived from `store.DepGraph`
- shared modules include `ReferencedBy` values in the rendered table

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestGenerateIndexPageListsModulesByDependencyDepth|TestGenerateIndexPageIncludesSharedUsedByColumn' -v
```

Expected: FAIL because `GenerateIndexPage` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func GenerateIndexPage(plan *store.NavPlan, graph store.DepGraph) string
```

Keep depth ordering deterministic and use stable tie-breaking by module ID.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Add failing composer orchestration tests before extending `generate`

**Files:**
- Modify: `internal/composer/renderer.go`
- Modify: `internal/composer/renderer_test.go`

**Step 1: Write the failing orchestration tests**

Add:

```go
func TestRunComposerWritesDocsIndexAndValidationReport(t *testing.T) {}
func TestRunComposerCreatesModuleAndSharedDirectories(t *testing.T) {}
```

Cover these behaviors:

- `RunComposer` creates the target docs directories
- `docs/index.md` is written
- validation report is written into artifacts as part of the Phase 5 flow

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestRunComposerWritesDocsIndexAndValidationReport|TestRunComposerCreatesModuleAndSharedDirectories' -v
```

Expected: FAIL because `RunComposer` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func RunComposer(cfg *config.Config, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) error
```

Requirements:

- build the symbol map
- ensure output directories exist
- call `CopyModuleDocs`
- write `docs/index.md`
- call `ValidateDocs(cfg.OutputDir)` and persist the report with `store.WriteValidationReport`

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Extend `generate` only after the Phase 5 entrypoint is stable

**Files:**
- Modify: `cmd/wikismit/generate.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing generate-integration tests**

Add:

```go
func TestGenerateCommandRunsComposerAfterPhase4(t *testing.T) {}
func TestGenerateCommandLoadsDepGraphForPhase5(t *testing.T) {}
```

Cover these behaviors:

- `generate` now reads `dep_graph.json`
- after Phase 4 it invokes Phase 5 and emits `docs/index.md`
- module-doc artifacts are still preserved as intermediate outputs

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestGenerateCommandRunsComposerAfterPhase4|TestGenerateCommandLoadsDepGraphForPhase5' -v
```

Expected: FAIL because `generate` still returns after Phase 4.

**Step 3: Write minimal implementation**

Extend `cmd/wikismit/generate.go` to:

- load `dep_graph.json` after Phase 1
- keep the existing Phase 4 filter for `Owner == "agent"`
- call `composer.RunComposer(cfg, &plan, idx, graph)` after Phase 4 succeeds

If Phase 4 returns partial-failure metadata, keep the existing summary behavior and only proceed to Phase 5 when the returned error contract allows it.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 6: Verify the full renderer/integration slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run composer tests**

Run:

```bash
go test ./internal/composer -v
```

Expected: PASS.

**Step 2: Run generate-command verification**

Run:

```bash
go test ./cmd/wikismit -run 'TestGenerateCommandRunsPhase4|TestGenerateCommandRunsComposerAfterPhase4|TestGenerateCommandLoadsDepGraphForPhase5' -v
```

Expected: PASS.

**Step 3: Run broader regression check**

Run:

```bash
go test ./...
```

Expected: PASS with no new failures.
