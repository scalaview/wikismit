# Epic 1 CLI Surface Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the root Cobra command, required subcommands, config bootstrap flow, and Epic 1 dry-run CLI behavior.

**Architecture:** Keep `cmd/wikismit` thin: every subcommand calls shared config bootstrap, `generate` prints resolved config, and the other commands remain intentional stubs for later epics.

**Tech Stack:** Go, Cobra, `internal/config`.

---

### Task 1: Create the root command

**Files:**
- Create: `cmd/wikismit/main.go`

**Step 1: Implement root command state**

Define package-level vars:

```go
var configPath string
var verbose bool
```

**Step 2: Add persistent flags**

Required flags:

- `--config` default `./config.yaml`
- `--verbose`

**Step 3: Add `Execute()` and `main()`**

Expected: binary entrypoint exists and compiles once subcommands are added.

### Task 2: Add shared config bootstrap helper

**Files:**
- Modify: `cmd/wikismit/main.go`

**Step 1: Add a helper**

```go
func loadAndValidateConfig() (*config.Config, error)
```

This helper should:

- call `config.LoadConfig(configPath)`
- call `cfg.Validate()`
- return joined errors to the caller

### Task 3: Create subcommand files and stubs

**Files:**
- Create: `cmd/wikismit/generate.go`
- Create: `cmd/wikismit/update.go`
- Create: `cmd/wikismit/plan.go`
- Create: `cmd/wikismit/validate.go`
- Create: `cmd/wikismit/build.go`

**Step 1: Implement `generate`**

Behavior:

- bootstrap config
- print resolved config in dry-run form
- exit 0

**Step 2: Implement `update`, `plan`, `validate`, `build`**

Behavior:

- bootstrap config
- print `not implemented`
- exit 0 on successful bootstrap

**Step 3: Register all subcommands with the root command**

### Task 4: Smoke test the CLI contract

**Files:**
- Verify only

**Step 1: Build**

Run:

```bash
go build -o ./wikismit ./cmd/wikismit
```

**Step 2: Verify help output**

Run:

```bash
./wikismit --help
```

Expected: lists `generate`, `update`, `plan`, `validate`, `build`.

**Step 3: Verify dry-run behavior**

Run:

```bash
./wikismit generate --config ./config.yaml
./wikismit update --config ./config.yaml
```

Expected:

- `generate` prints resolved config
- `update` prints `not implemented`
- no panic

**Step 4: Commit**

```bash
git add cmd/wikismit
git commit -m "feat: add Epic 1 CLI scaffold"
```
