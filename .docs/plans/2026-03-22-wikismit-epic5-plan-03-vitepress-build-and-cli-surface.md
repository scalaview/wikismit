# Epic 5 VitePress Build and CLI Surface Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generate `docs/.vitepress/config.ts`, copy optional site assets, and replace the `wikismit build` stub with a real VitePress build command.

**Architecture:** First lock the config-template output shape with string-level tests. Then add logo-copy behavior and extend the Phase 5 composer entrypoint to write the VitePress config automatically. Only after those deterministic pieces are stable should the `build` command be wired to environment checks and `vitepress build` execution.

**Tech Stack:** Go, standard `testing`, `text/template`, `os`, `os/exec`, `path/filepath`, existing `internal/config`, existing Cobra command tests.

---

### Task 1: Add failing VitePress config tests before writing the template

**Files:**
- Create: `internal/composer/vitepress.go`
- Create: `internal/composer/vitepress_test.go`

**Step 1: Write the failing config tests**

Add:

```go
func TestGenerateVitePressConfigBuildsModulesAndSharedSidebarGroups(t *testing.T) {}
func TestGenerateVitePressConfigUsesSiteTitleOrRepoNameFallback(t *testing.T) {}
func TestGenerateVitePressConfigIncludesEditLinkOnlyWhenRepoURLPresent(t *testing.T) {}
```

Cover these behaviors:

- sidebar has `Modules` and `Shared` groups
- title comes from `cfg.Site.Title`, otherwise repo directory name
- `editLink` renders only when `cfg.Site.RepoURL` is set

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestGenerateVitePressConfigBuildsModulesAndSharedSidebarGroups|TestGenerateVitePressConfigUsesSiteTitleOrRepoNameFallback|TestGenerateVitePressConfigIncludesEditLinkOnlyWhenRepoURLPresent' -v
```

Expected: FAIL because `GenerateVitePressConfig` does not exist.

**Step 3: Write minimal implementation**

Implement in `internal/composer/vitepress.go`:

```go
func GenerateVitePressConfig(plan *store.NavPlan, graph store.DepGraph, cfg *config.Config) (string, error)
```

Use `text/template`, not string concatenation.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Lock logo handling and output-validity checks before composer integration

**Files:**
- Modify: `internal/composer/vitepress.go`
- Modify: `internal/composer/vitepress_test.go`

**Step 1: Write the failing asset/config tests**

Add:

```go
func TestWriteVitePressAssetsCopiesLogoWhenConfigured(t *testing.T) {}
func TestGenerateVitePressConfigOmitsTemplateArtifactsFromOutput(t *testing.T) {}
```

Cover these behaviors:

- configured logo gets copied to `docs/public/logo.png`
- rendered config text has no leftover `{{` template markers

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestWriteVitePressAssetsCopiesLogoWhenConfigured|TestGenerateVitePressConfigOmitsTemplateArtifactsFromOutput' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Add helper(s) that:

- create `docs/.vitepress/` and `docs/public/` when needed
- copy the logo file only when `cfg.Site.Logo` is set
- keep config generation and file-copy behavior deterministic

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Extend `RunComposer` to emit VitePress config after Markdown output exists

**Files:**
- Modify: `internal/composer/renderer.go`
- Modify: `internal/composer/renderer_test.go`
- Modify: `internal/composer/vitepress_test.go`

**Step 1: Write the failing composer/VitePress integration test**

Add:

```go
func TestRunComposerWritesVitePressConfigAndOptionalLogo(t *testing.T) {}
```

Cover these behaviors:

- `docs/.vitepress/config.ts` is written during Phase 5
- configured logo is copied into `docs/public/logo.png`

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run TestRunComposerWritesVitePressConfigAndOptionalLogo -v
```

Expected: FAIL because `RunComposer` does not yet emit VitePress files.

**Step 3: Write minimal implementation**

Extend `RunComposer` so that after Markdown docs and validation report are written, it:

- renders the VitePress config
- writes `docs/.vitepress/config.ts`
- copies the optional logo into `docs/public/logo.png`

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Replace the `build` command stub with real prerequisite checks

**Files:**
- Modify: `cmd/wikismit/build.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing command tests**

Add:

```go
func TestBuildCommandErrorsWhenVitePressConfigIsMissing(t *testing.T) {}
func TestBuildCommandErrorsWhenNodeIsUnavailable(t *testing.T) {}
```

Cover these behaviors:

- missing `docs/.vitepress/config.ts` returns an error instructing the user to run `wikismit generate` first
- missing `node` in `PATH` returns an error about Node.js 20+

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestBuildCommandErrorsWhenVitePressConfigIsMissing|TestBuildCommandErrorsWhenNodeIsUnavailable' -v
```

Expected: FAIL because the command still returns `not implemented`.

**Step 3: Write minimal implementation**

Replace the stub with a real command path that:

- verifies `docs/.vitepress/config.ts` exists under `cfg.OutputDir`
- checks `node` availability with `exec.LookPath`
- returns clear errors before shelling out when prerequisites are missing

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Add execution-path coverage for npm install and VitePress build

**Files:**
- Modify: `cmd/wikismit/build.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing execution tests**

Add:

```go
func TestBuildCommandInstallsVitePressWhenNodeModulesMissing(t *testing.T) {}
func TestBuildCommandSkipsInstallWhenNodeModulesAlreadyExist(t *testing.T) {}
```

Use injectable command runners so tests can assert the intended commands without invoking a real network install.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestBuildCommandInstallsVitePressWhenNodeModulesMissing|TestBuildCommandSkipsInstallWhenNodeModulesAlreadyExist' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement command execution so `wikismit build`:

- runs `npm install -D vitepress` in the docs directory only when `node_modules/` is absent
- runs `npx vitepress build docs`
- logs a stable success message on completion

Keep the command runner injectable for tests.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 6: Verify the full VitePress/build slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run VitePress/composer tests**

Run:

```bash
go test ./internal/composer -run 'TestGenerateVitePressConfig|TestWriteVitePressAssets|TestRunComposerWritesVitePressConfigAndOptionalLogo' -v
```

Expected: PASS.

**Step 2: Run build-command tests**

Run:

```bash
go test ./cmd/wikismit -run 'TestBuildCommand' -v
```

Expected: PASS.

**Step 3: Run broader regression check**

Run:

```bash
go test ./...
```

Expected: PASS with no new failures.
