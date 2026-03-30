# wikismit — Epic 8A: Runtime Correctness + Command Hardening

**Status:** `todo`
**Depends on:** Epic 6
**Goal:** Fresh-run documentation generation works from empty artifacts, build-time path handling is predictable, and the LLM client enforces basic request correctness before network calls.
**Spec refs:** code-review-oop-refactoring.md §2.1, §3.1-§3.2, §5.2-§5.4

---

## S8A.1 — Fresh-run full generate orchestration

**Status:** `todo`

**Description:**
Make `wikismit generate` a true from-scratch entrypoint. It must run the same five-phase flow already used by incremental fallback instead of assuming `nav_plan.json` and `shared_context.json` already exist.

**Acceptance criteria:**
- Running `wikismit generate` against `testdata/sample_repo` with an empty artifacts directory writes `file_index.json`, `dep_graph.json`, `nav_plan.json`, `shared_context.json`, `artifacts/module_docs/*.md`, and `output/index.md`
- `cmd/wikismit/generate.go` no longer reads `nav_plan.json` or `shared_context.json` before they are produced in the same run
- The full-generate orchestration used by `generate` and the fallback path used by `update` come from one shared implementation path
- Existing `update` fallback behavior remains unchanged

**Files to modify:**
```
cmd/wikismit/generate.go
cmd/wikismit/main_test.go
internal/pipeline/incremental.go
internal/pipeline/incremental_test.go
```

### Subtasks

#### S8A.1.1 — Add failing fresh-run generate coverage

- Add a CLI-level test that starts with an empty artifacts directory and runs `generate`
- Use the existing mock-client pattern from `cmd/wikismit/main_test.go`
- Assert that `nav_plan.json`, `shared_context.json`, module docs, and composed docs are written in one run

#### S8A.1.2 — Extract one shared full-generate orchestration path

- Introduce the smallest shared helper needed so `generate` and incremental fallback call the same full-pipeline implementation
- Keep Phase 1, planner, preprocessor, agent fan-out, and composer ordering identical to the current fallback implementation
- Avoid copying orchestration logic into `cmd/wikismit/generate.go`

#### S8A.1.3 — Route `generate` through the shared helper

- Update `runGenerate` so it no longer manually reads artifacts produced by later phases
- Preserve existing command-level client creation behavior
- Keep command output behavior unchanged unless a test requires a specific message update

#### S8A.1.4 — Verify fresh-run command behavior

- Run focused command tests covering both `generate` and `update` fallback
- Re-run `go test ./cmd/wikismit ./internal/pipeline -v`

---

## S8A.2 — Build output directory hardening

**Status:** `todo`

**Description:**
Tighten `wikismit build` input handling around `cfg.OutputDir` so command execution always happens in a validated directory with clearer failure modes. Treat this as local CLI hardening, not as a generalized sandbox.

**Acceptance criteria:**
- `wikismit build` returns a clear error when `cfg.OutputDir` resolves to a file instead of a directory
- Command execution for `npm` and `npx` always uses the validated output directory path
- Existing VitePress-config detection behavior remains intact
- Existing success paths for package-managed and bare `vitepress` builds remain unchanged

**Files to modify:**
```
cmd/wikismit/build.go
cmd/wikismit/main_test.go
```

### Subtasks

#### S8A.2.1 — Add failing build validation tests

- Add tests covering `cfg.OutputDir` pointing at a regular file
- Add tests proving build still returns the existing “run wikismit generate first” error when VitePress config is absent

#### S8A.2.2 — Add output directory validation helper

- Resolve `cfg.OutputDir` to an absolute path before using it as `cmd.Dir`
- Verify the resolved path exists and is a directory before invoking `npm` or `npx`
- Return explicit errors for invalid paths instead of relying on subprocess failures

#### S8A.2.3 — Preserve current build branching

- Keep the existing `package.json` / `node_modules` decision tree intact
- Preserve `npm run docs:build` behavior when a project package script exists
- Preserve `npx vitepress build docs` fallback when no package manifest exists

#### S8A.2.4 — Verify build command regressions

- Run focused build command tests
- Re-run `go test ./cmd/wikismit -run 'TestBuild|TestRoot' -v`

---

## S8A.3 — LLM request validation and loop cleanup

**Status:** `todo`

**Description:**
Clean up the `for true` completion loop and reject obviously invalid completion requests before any network call is made. This keeps the client behavior predictable while staying compatible with the current OpenAI-compatible transport.

**Acceptance criteria:**
- `internal/llm/client.go` no longer uses `for true`
- `CompletionRequest` rejects empty `UserMsg`
- `CompletionRequest` rejects non-positive `MaxTokens`
- Invalid requests fail locally before `CreateChatCompletion` is invoked
- Existing multi-part `FinishReasonLength` continuation behavior still works

**Files to modify:**
```
internal/llm/types.go
internal/llm/client.go
internal/llm/client_test.go
```

### Subtasks

#### S8A.3.1 — Add failing client validation tests

- Add tests for empty `UserMsg`
- Add tests for `MaxTokens <= 0`
- Assert the client returns a local error without needing a live provider call

#### S8A.3.2 — Replace `for true` with explicit loop semantics

- Keep the current truncated-output continuation behavior
- Make the loop exit condition explicit and Go-idiomatic
- Preserve the accumulated-content behavior across partial completions

#### S8A.3.3 — Add request validation to the client boundary

- Add a small validation helper on `CompletionRequest` or the client boundary
- Call validation before building the provider request payload
- Return consistent `LLMError`-compatible or standard errors that tests can assert on

#### S8A.3.4 — Verify client behavior

- Run `go test ./internal/llm -v`
- Re-run `go test ./...` after all Epic 8A changes land
