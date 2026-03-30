# Epic 8A.2 Build Output Directory Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Validate `cfg.OutputDir` before any `wikismit build` subprocess work so build failures are explicit and `npm` / `npx` always run inside a known directory.

**Architecture:** Add one narrow helper in `cmd/wikismit/build.go` that resolves the output directory to an absolute path, verifies it exists, and verifies it is a directory. Reuse that validated path for `existingConfigPath`, `filepath.Join`, and every `runCommand` call, while preserving the current npm/npx branch logic and existing missing-config guidance.

**Tech Stack:** Go, Cobra CLI, `os.Stat`, `filepath.Abs`, package-local command seams (`lookPath`, `runCommand`), standard `testing`.

---

### Task 1: Add failing validation and branch-preservation tests

**Files:**
- Modify: `cmd/wikismit/main_test.go:852-1033`
- Test: `cmd/wikismit/main_test.go`

- [ ] **Step 1: Add a helper-level RED test for file-vs-directory validation**

Add a package-local test that will call the new helper directly, for example:

```go
func TestResolveBuildOutputDirRejectsFile(t *testing.T) {
    filePath := filepath.Join(t.TempDir(), "docs-file")
    if err := os.WriteFile(filePath, []byte("not a directory"), 0o644); err != nil {
        t.Fatal(err)
    }

    _, err := resolveBuildOutputDir(filePath)
    if err == nil || !strings.Contains(err.Error(), "directory") {
        t.Fatalf("resolveBuildOutputDir() error = %v, want directory validation error", err)
    }
}
```

Use a helper name that reads well in `build.go` and keep it package-local.

- [ ] **Step 2: Add a helper-level RED test for absolute-path normalization**

Add `TestResolveBuildOutputDirReturnsAbsoluteDirectoryPath`. Create a real directory, convert it into a relative path from the current package working directory, then assert the helper returns the original absolute path.

Use this exact pattern so the test is deterministic:

```go
wd, err := os.Getwd()
if err != nil {
    t.Fatal(err)
}
absDir := filepath.Join(t.TempDir(), "docs")
if err := os.MkdirAll(absDir, 0o755); err != nil {
    t.Fatal(err)
}
relativeDir, err := filepath.Rel(wd, absDir)
if err != nil {
    t.Fatal(err)
}

got, err := resolveBuildOutputDir(relativeDir)
if err != nil {
    t.Fatalf("resolveBuildOutputDir() error = %v", err)
}
if got != absDir {
    t.Fatalf("resolveBuildOutputDir() = %q, want %q", got, absDir)
}
```

This locks the “validated output path” acceptance criterion without depending on subprocess behavior.

- [ ] **Step 3: Add a command-level RED test for the bare npx branch**

Add `TestBuildCommandFallsBackToNpxWhenPackageJSONIsMissing`. Set up:

1. a valid output directory
2. `.vitepress/config.mts`
3. **no** `package.json`
4. **no** `node_modules`

Override `lookPath` and `runCommand`, then assert the captured command sequence is exactly:

```text
npm install -D vitepress
npx vitepress build docs
```

Both commands must use the validated output directory argument.

- [ ] **Step 4: Re-run existing build tests plus the new ones to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestResolveBuildOutputDir|TestBuildCommand(ErrorsWhenVitePressConfigIsMissing|ErrorsWhenNodeIsUnavailable|InstallsVitePressWhenNodeModulesMissing|SkipsInstallWhenNodeModulesAlreadyExist|FallsBackToNpxWhenPackageJSONIsMissing)' -v
```

Expected: FAIL because the helper does not exist yet and the npx fallback path is not explicitly covered.

- [ ] **Step 5: Commit the RED test slice**

```bash
git add cmd/wikismit/main_test.go
git commit -m "test: lock build output directory validation"
```

### Task 2: Implement one validated output-directory helper and thread it through build

**Files:**
- Modify: `cmd/wikismit/build.go:3-75`

- [ ] **Step 1: Add the path-resolution helper**

Implement a package-local helper such as:

```go
func resolveBuildOutputDir(outputDir string) (string, error) {
    absPath, err := filepath.Abs(outputDir)
    if err != nil {
        return "", fmt.Errorf("resolve output directory %q: %w", outputDir, err)
    }

    info, err := os.Stat(absPath)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return "", fmt.Errorf("output directory %q does not exist", absPath)
        }
        return "", fmt.Errorf("stat output directory %q: %w", absPath, err)
    }
    if !info.IsDir() {
        return "", fmt.Errorf("output directory %q is not a directory", absPath)
    }
    return absPath, nil
}
```

Keep this helper local to `cmd/wikismit/build.go`; do not widen scope into a shared sandbox utility.

- [ ] **Step 2: Validate before checking VitePress config**

At the top of `newBuildCmd().RunE`, do:

```go
validatedOutputDir, err := resolveBuildOutputDir(cfg.OutputDir)
if err != nil {
    return err
}
```

Then call `existingConfigPath(validatedOutputDir)`. This order ensures “output path is a file” fails with a clear message instead of a later `os.Stat` / subprocess error.

- [ ] **Step 3: Reuse the validated path for every command-side file and subprocess call**

Replace these uses of `cfg.OutputDir`:

```go
filepath.Join(cfg.OutputDir, "node_modules")
filepath.Join(cfg.OutputDir, "package.json")
runCommand(cfg.OutputDir, ...)
```

with:

```go
filepath.Join(validatedOutputDir, "node_modules")
filepath.Join(validatedOutputDir, "package.json")
runCommand(validatedOutputDir, ...)
```

Do **not** change the branch logic itself:

- `npm install` when `package.json` exists and `node_modules` is missing
- `npm install -D vitepress` when `package.json` is missing and `node_modules` is missing
- `npm run docs:build` when `package.json` exists
- `npx vitepress build docs` otherwise

- [ ] **Step 4: Keep user-facing success behavior stable**

Unless a test explicitly requires otherwise, keep the final success message format unchanged:

```go
Build complete: VitePress site built from %s
```

Prefer leaving `%s` as `cfg.OutputDir` to avoid an unnecessary user-visible switch from relative to absolute paths.

- [ ] **Step 5: Run the targeted build suite to confirm GREEN**

Run the command from Task 1 Step 4 again.

Expected: PASS.

### Task 3: Re-check both build branches and finish verification

**Files:**
- Modify only if a test-driven fix is required: `cmd/wikismit/main_test.go`

- [ ] **Step 1: Strengthen existing command-capture assertions**

Update `TestBuildCommandInstallsVitePressWhenNodeModulesMissing` and `TestBuildCommandSkipsInstallWhenNodeModulesAlreadyExist` so they assert both:

1. the command string is correct
2. the captured `dir` equals the validated output directory used by the helper

Because `t.TempDir()` already returns an absolute path, these tests can assert the captured directory directly.

- [ ] **Step 2: Run focused build and root command regression**

Run:

```bash
go test ./cmd/wikismit -run 'TestBuild|TestRoot' -v
```

Expected: PASS.

- [ ] **Step 3: Run the full CLI package suite**

Run:

```bash
go test ./cmd/wikismit -v
```

Expected: PASS.

- [ ] **Step 4: Spot-check the untouched missing-config behavior**

Re-run just:

```bash
go test ./cmd/wikismit -run 'TestBuildCommandErrorsWhenVitePressConfigIsMissing' -v
```

Expected: PASS with stderr still containing `run wikismit generate first`.

- [ ] **Step 5: Commit the hardening slice**

```bash
git add cmd/wikismit/build.go cmd/wikismit/main_test.go
git commit -m "fix: validate build output directory before subprocesses"
```
