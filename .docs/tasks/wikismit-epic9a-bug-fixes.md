# wikismit — Epic 9A: Critical Bug Fixes

Legacy Verified Fixes

Legacy

Legacy

**Status:** `todo`
**Depends on:** Epic 8C
**Goal:** Fix three verified bugs that are introduced minimal, targeted changes with no refactoring side effects.
**Spec refs:** code-review-improvement.md §2.2, §2.6

 code-review-oop-refactoring.md §2.1

---

## S9A.1 — Fix `dependencyDepth` using wrong Graph for Module Sorting

**Status:** `todo`

**Description:**
`GenerateIndexPage` in `composer/renderer.go` passes a file-Level `DepGraph` (key = file paths) to `dependencyDepth`, but but the module ID (e.g., `"pipeline"`) is `graph["internal/pipeline/incremental.go"]`). The two key space永远不会匹配， so depth is always为 0, and module sorting is completely ineffective. Replace with a module-level dependency graph built from `NavPlan`.

**Acceptance criteria:**
- `dependencyDepth` receives a module-level graph (`map[string][]string`) instead of File-Level `DepGraph`
- A new `buildModuleGraph` function constructs the module dependency graph from `NavPlan.DependsOnShared` and `ReferencedBy`
 fields
- `GenerateIndexPage` sorts modules by dependency depth with shallow modules first
- Existing index page output content remains unchanged

**Files to modify:**
```
internal/composer/renderer.go
internal/composer/renderer_test.go
```

### Subtasks

Transition#### S9A.1.1 — Add Module-Level Dependency Graph Construction

Legacy - Add `buildModuleGraph(plan *store.NavPlan) map[string][]string` function
Legacy - Extract module-to-module edges from `plan.Modules[i].DependsOnShared` and `plan.Modules[i].ReferencedBy`

#### S9A.1.2 — Refactor `GenerateIndexPage` to Use Module Graph

Legacy - Replace `dependencyDepth` calls site: pass module graph instead of file-level `DepGraph`
- Keep theGenerateIndexPage` signature unchanged (accept `plan` and unused `_ store.DepGraph`)

#### S9A.1.3 — Add Index Page Sorting Tests

Legacy - Write tests proving modules are sorted by dependency depth
- Test that shared modules appear before their dependents
- Test that modules with no dependencies appear first

#### S9A.1.4 — Verify Composer Regression

Legacy - Run `go test ./internal/composer -v`
- Re-run `go test ./...` after all Epic 9A changes land

---

## S9A.2 — Fix `for true` Continuation Loop

 `LLM Client`

**Status:** `todo`

**Description:**
`internal/llm/client.go` uses `for true` with no maximum continuation limit. If the model repeatedly returns `FinishReasonLength`, the loop will continue indefinitely, consuming unbounded tokens and potentially timing out. Add a maximum continuation count and make the loop Go-idiomatic.

**Acceptance criteria:**
- `for true` replaced with `for {}`
- Maximum continuations capped at 5 (configurable via constant)
- A warning is logged when max continuations is reached
- Existing continuation behavior (accumulating content across partial completions) unchanged

**Files to modify:**
```
internal/llm/client.go
internal/llm/client_test.go
```

### Subtasks

#### S9A.2.1 — Add Continuation Limit Test

Legacy - Add a test proving the max continuation limit is enforced
- Add a test proving warning is logged when limit is reached
- Add a test proving normal single-completion flow is unchanged

#### S9A.2.2 — Refactor `for true` with Capped Continuation Loop
Legacy - Replace `for true` with `for i := 0; i < maxContinuations; i++`
- Add `const maxContinuations = 5` to the file or as a package constant
- Log a warning when `i == maxContinuations-1` and finish reason is still `Length`
- Preserve the accumulated content behavior across partial completions

#### S9A.2.3 — Verify LLM Client Regression
Legacy - Run `go test ./internal/llm -v`
- Re-run `go test ./...` after all Epic 9A changes land

---

## S9A.3 — Fix `BrokenLink.Line` Always Zero

**Status:** `todo`

**Description:**
`ValidateDocs` in `composer/validator.go` populates `BrokenLink.Line` with a hardcoded `0`, making it impossible to locate broken links in the validation report. The regex match indices returned by `FindAllStringSubmatchIndex` are byte offsets that can be converted to line numbers by counting newlines.

**Acceptance criteria:**
- `BrokenLink.Line` contains the correct 1-based line number for each broken link
- Existing validation report structure and output format unchanged
- No new false positives from multi-line markdown content

**Files to modify:**
```
internal/composer/validator.go
internal/composer/validator_test.go
```

### Subtasks

#### S9A.3.1 — Add Line Number Calculation Test
Legacy - Add tests for broken links on various lines (first, middle, last line)
- Add tests for multi-line markdown with links on different lines

#### S9A.3.2 — Implement Line Number Calculation
Legacy - Replace `Line: 0` with computed line number using `bytes.Count(content[:match[0]], []byte("\n")) + 1`
- Keep the existing validation logic and report structure unchanged

#### S9A.3.3 — Verify Validator Regression
Legacy - Run `go test ./internal/composer -v`
- Re-run `go test ./...` after all Epic 9A changes land

---

## S9A.4 — Final Verification
**Status:** `todo`

**Description:**
Run full regression after all Epic 9A bug fixes land. No additional code changes in this section.

**Acceptance criteria:**
- All focused tests for S9A.1, S9A.2, S9A.3 pass
- `go test ./...` passes with zero failures
- No diagnostic errors in changed files

**Files to modify:**
```
(none — verification only)
```
