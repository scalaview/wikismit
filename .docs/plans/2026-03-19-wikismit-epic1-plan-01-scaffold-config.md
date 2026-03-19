# Epic 1 Scaffold and Config Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create the Go module, project skeleton, config schema, config loading, env resolution, validation, and config unit tests.

**Architecture:** Establish the repository shape first, then build `internal/config` as a standalone unit with tests before wiring any CLI code. Keep defaults and validation centralized in `internal/config/config.go` so every command can share the same bootstrap logic.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, standard `testing`, `errors.Join`.

---

### Task 1: Create the module and directory skeleton

**Files:**
- Create: `go.mod`
- Modify: `.gitignore`
- Create: `cmd/wikismit/.gitkeep`
- Create: `internal/analyzer/.gitkeep`
- Create: `internal/analyzer/lang/.gitkeep`
- Create: `internal/planner/.gitkeep`
- Create: `internal/preprocessor/.gitkeep`
- Create: `internal/agent/.gitkeep`
- Create: `internal/composer/.gitkeep`
- Create: `internal/llm/.gitkeep`
- Create: `internal/config/.gitkeep`
- Create: `internal/log/.gitkeep`
- Create: `pkg/gitdiff/.gitkeep`
- Create: `pkg/store/.gitkeep`
- Create: `artifacts/module_docs/.gitkeep`
- Create: `artifacts/cache/.gitkeep`
- Create: `docs/modules/.gitkeep`
- Create: `docs/shared/.gitkeep`
- Create: `docs/.vitepress/.gitkeep`
- Create: `examples/github/.gitkeep`

**Step 1: Initialize the module**

Run:

```bash
go mod init github.com/scalaview/wikismit
```

**Step 2: Create the directory tree from spec §6**

Run:

```bash
mkdir -p cmd/wikismit internal/{analyzer/lang,planner,preprocessor,agent,composer,llm,config,log} pkg/{gitdiff,store} artifacts/{module_docs,cache} docs/{modules,shared,.vitepress} examples/github
touch cmd/wikismit/.gitkeep internal/analyzer/.gitkeep internal/analyzer/lang/.gitkeep internal/planner/.gitkeep internal/preprocessor/.gitkeep internal/agent/.gitkeep internal/composer/.gitkeep internal/llm/.gitkeep internal/config/.gitkeep internal/log/.gitkeep pkg/gitdiff/.gitkeep pkg/store/.gitkeep artifacts/module_docs/.gitkeep artifacts/cache/.gitkeep docs/modules/.gitkeep docs/shared/.gitkeep docs/.vitepress/.gitkeep examples/github/.gitkeep
```

**Step 3: Add initial dependencies**

Run:

```bash
go get github.com/spf13/cobra gopkg.in/yaml.v3
go mod tidy
```

Expected: `go mod tidy` succeeds.

**Step 4: Update `.gitignore`**

Add:

```gitignore
.omc
artifacts/*.json
artifacts/cache/
artifacts/module_docs/*.md
docs/.vitepress/dist/
```

**Step 5: Commit**

```bash
git add go.mod go.sum .gitignore cmd internal pkg artifacts docs examples
git commit -m "chore: scaffold Epic 1 project layout"
```

### Task 2: Define config types and default values

**Files:**
- Create: `internal/config/config.go`
- Create: `config.yaml`

**Step 1: Write the config structs**

Add `Config`, `LLMConfig`, `AnalysisConfig`, `AgentConfig`, `CacheConfig`, and `SiteConfig` with yaml tags matching spec §11.

**Step 2: Add default helpers**

Implement:

```go
func defaultConfig() Config
func applyDefaults(cfg *Config)
```

**Step 3: Write the template config file**

Include all top-level sections: `repo_path`, `output_dir`, `artifacts_dir`, `llm`, `analysis`, `agent`, `cache`, `site`.

**Step 4: Commit**

```bash
git add internal/config/config.go config.yaml
git commit -m "feat: define Epic 1 config schema and defaults"
```

### Task 3: Add failing tests for config loading and validation

**Files:**
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests**

Add:

```go
func TestLoadConfigLoadsValidYAML(t *testing.T) {}
func TestLoadConfigReturnsErrorWhenAPIKeyEnvMissing(t *testing.T) {}
func TestLoadConfigReadsResolvedAPIKey(t *testing.T) {}
func TestValidateRejectsBadConcurrency(t *testing.T) {}
func TestValidateRejectsMissingRepoPath(t *testing.T) {}
```

**Step 2: Run tests to confirm failure**

Run:

```bash
go test ./internal/config -v
```

Expected: FAIL because loading and validation are not implemented yet.

### Task 4: Implement loading, env resolution, and validation

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Implement `LoadConfig(path string) (*Config, error)`**

Requirements:

- read YAML
- unmarshal config
- apply defaults
- resolve `os.Getenv(cfg.LLM.APIKeyEnv)`
- store unexported `resolvedAPIKey`

**Step 2: Implement `APIKey()`**

```go
func (c LLMConfig) APIKey() string
```

**Step 3: Implement `Validate()`**

Rules:

- repo path exists and is a directory
- `LLM.APIKeyEnv` non-empty
- concurrency in `[1, 32]`
- shared threshold `>= 1`

Use `errors.Join`.

**Step 4: Run tests**

Run:

```bash
go test ./internal/config -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add config loading and validation"
```
