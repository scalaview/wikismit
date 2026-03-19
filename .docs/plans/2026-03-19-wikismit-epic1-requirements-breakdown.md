# wikismit Epic 1 Requirements Breakdown

## Goal

Split Epic 1 into smaller requirement slices before writing implementation plans.

Epic 1 target from `.docs/tasks/wikismit-epic1.md` is:

- runnable Go CLI
- config loading + validation
- real OpenAI-compatible LLM completion
- retry wrapper + logging
- mock LLM client for later epics
- artifact store layer used by later phases

## Requirement slices

### R1 — Repository scaffold and dependency baseline

Source: `S1.1.1`

Includes:

- `go mod init github.com/scalaview/wikismit`
- directory skeleton from spec §6
- initial deps: `cobra`, `yaml.v3`
- `.gitignore` updates for artifacts/build output

Output:

- repo layout exists
- `go mod tidy` passes

### R2 — Config model, defaults, env resolution, validation

Source: `S1.1.2`, `S1.1.3`, `S1.1.4`, `S1.1.6`

Includes:

- `Config` / `LLMConfig` / `AnalysisConfig` / `AgentConfig` / `CacheConfig` / `SiteConfig`
- `LoadConfig(path)`
- default values from spec §11
- env var resolution for `api_key_env`
- `APIKey()` accessor
- `Validate()` with joined errors
- unit tests for happy path and invalid config cases

Output:

- `internal/config` is independently testable
- `config.yaml` shape matches the spec

### R3 — CLI surface and dry-run behavior

Source: `S1.1.5` + Epic 1 acceptance criteria

Includes:

- root Cobra command
- persistent `--config`
- persistent `--verbose`
- subcommands: `generate`, `update`, `plan`, `validate`, `build`
- all commands bootstrap config before running
- `generate` prints resolved config for dry-run
- all other commands stay stubbed for now

Output:

- `wikismit --help` is correct
- no panic during CLI bootstrap

### R4 — LLM client core

Source: `S1.2`

Includes:

- `CompletionRequest`
- `Client` interface with `Complete`
- `openAIClient`
- `NewClient(cfg config.LLMConfig)`
- OpenAI-compatible base URL support
- `LLMError`
- `httptest` coverage for success, 401, 500, timeout

Output:

- real endpoint can be hit through one client surface

### R5 — Retry wrapper and logger abstraction

Source: `S1.3`

Includes:

- `retryingClient`
- exponential backoff with jitter
- context-cancellable wait
- `internal/log/log.go`
- `slog`-backed logger

Output:

- retry policy is isolated from base client

### R6 — Mock LLM client

Source: `S1.4`

Includes:

- deterministic response sequence
- optional per-call error injection
- call recording helpers

Output:

- later epics can write unit tests without real LLM calls

### R7 — Artifact store layer and Epic 1 verification

Source: `S1.5`

Includes:

- artifact types
- atomic JSON write helper
- typed read/write helpers
- `ErrArtifactNotFound`
- round-trip and atomicity tests
- final Epic 1 smoke verification

Output:

- storage contract exists before analyzer/planner/agent work starts

## Dependency order

Implement in this order:

1. `R1` scaffold
2. `R2` config
3. `R3` CLI
4. `R4` LLM core
5. `R5` retry + logging
6. `R6` mock client
7. `R7` store + final verification

## Implementation document map

- `R1 + R2` → `.docs/plans/2026-03-19-wikismit-epic1-plan-01-scaffold-config.md`
- `R3` → `.docs/plans/2026-03-19-wikismit-epic1-plan-02-cli-surface.md`
- `R4` → `.docs/plans/2026-03-19-wikismit-epic1-plan-03-llm-client.md`
- `R5 + R6` → `.docs/plans/2026-03-19-wikismit-epic1-plan-04-retry-mock.md`
- `R7` → `.docs/plans/2026-03-19-wikismit-epic1-plan-05-artifact-store-verification.md`

## Out of scope for Epic 1

Do not implement yet:

- analyzer logic
- planner logic
- preprocessor logic
- agent fan-out
- composer / vitepress generation
- git diff / incremental update logic
- streaming responses
