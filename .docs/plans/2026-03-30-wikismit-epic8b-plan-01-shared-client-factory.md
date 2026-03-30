# Epic 8B.1 Shared Command-Side Client Factory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three duplicated client-construction patterns in `generate`, `plan`, and `update` with a single shared helper in `cmd/wikismit/helpers.go`, while preserving each command's existing test-factory seam.

**Architecture:** Introduce a `resolveClient` helper that accepts a command-specific factory override and the shared config, then falls back to `llm.NewClient(cfg.LLM)`. Each command keeps its package-level factory variable for test overriding, but delegates the nil-check + `llm.NewClient` creation to the helper. The update command's two-tier fallback (updateFactory → agentFactory → NewClient) becomes an explicit ordered override chain.

**Tech Stack:** Go, Cobra CLI, `internal/llm`, `pkg/config`, standard `testing`.

---

### Task 1: Add failing tests for the shared client resolution path

**Files:**
- Modify: `cmd/wikismit/main_test.go`

- [ ] **Step 1: Add a test proving the shared helper resolves a nil factory to a real client**

Add a test `TestResolveClientCreatesLLMClientWhenFactoryReturnsNil` that:
1. Calls a `resolveClient` function with a nil-returning factory and a valid config
2. Asserts the returned client is non-nil and is the `*llm.realClient` type (or just non-nil)

This test will not compile until `resolveClient` is introduced in Task 2.

```go
func TestResolveClientCreatesLLMClientWhenFactoryReturnsNil(t *testing.T) {
    cfg := configpkg.Config{LLM: configpkg.LLMConfig{Provider: "openai", Model: "gpt-4"}}
    client, err := resolveClient(func() llm.Client { return nil }, &cfg)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if client == nil {
        t.Fatal("expected non-nil client")
    }
}
```

Expected: **compile error** — `resolveClient` undefined.

- [ ] **Step 2: Add a test proving a non-nil factory override bypasses NewClient**

```go
func TestResolveClientUsesFactoryOverrideWhenProvided(t *testing.T) {
    cfg := configpkg.Config{LLM: configpkg.LLMConfig{Provider: "openai", Model: "gpt-4"}}
    mock := llm.NewMockClient("test response")
    client, err := resolveClient(func() llm.Client { return mock }, &cfg)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if client != mock {
        t.Fatal("expected factory override to be used")
    }
}
```

Expected: **compile error** — `resolveClient` undefined.

- [ ] **Step 3: Add a test proving update's two-tier fallback chain**

```go
func TestResolveClientChainsUpdateFallbackToAgentFactory(t *testing.T) {
    cfg := configpkg.Config{LLM: configpkg.LLMConfig{Provider: "openai", Model: "gpt-4"}}
    agentMock := llm.NewMockClient("agent response")
    // update factory returns nil, agent factory returns mock
    client, err := resolveClientWithFallback(
        func() llm.Client { return nil },   // updateFactory
        func() llm.Client { return agentMock }, // agentFactory
        &cfg,
    )
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if client != agentMock {
        t.Fatal("expected agent factory fallback")
    }
}
```

Expected: **compile error** — `resolveClientWithFallback` undefined.

- [ ] **Step 4: Verify RED**

Run:

```bash
go test ./cmd/wikismit -run "TestResolveClient|TestResolveClientChains" -v
```

Expected: **compile error** (`undefined: resolveClient`). This confirms the tests are properly RED.

---

### Task 2: Introduce the shared client-construction helper

**Files:**
- Modify: `cmd/wikismit/helpers.go`

- [ ] **Step 1: Add `resolveClient` function to helpers.go**

Append after the existing helper functions:

```go
// resolveClient returns a client from the given factory, or creates one using
// the provided config. Returns an error only when llm.NewClient fails.
func resolveClient(factory func() llm.Client, cfg *configpkg.Config) (llm.Client, error) {
    if client := factory(); client != nil {
        return client, nil
    }
    return llm.NewClient(cfg.LLM)
}
```

- [ ] **Step 2: Add `resolveClientWithFallback` for the update command's two-tier chain**

```go
// resolveClientWithFallback tries primaryFactory, then fallbackFactory, then
// creates a client from config. Used by update command which cascades from
// updateClientFactory to agentClientFactory.
func resolveClientWithFallback(primaryFactory, fallbackFactory func() llm.Client, cfg *configpkg.Config) (llm.Client, error) {
    if client := primaryFactory(); client != nil {
        return client, nil
    }
    return resolveClient(fallbackFactory, cfg)
}
```

- [ ] **Step 3: Verify GREEN on Task 1 tests**

Run:

```bash
go test ./cmd/wikismit -run "TestResolveClient|TestResolveClientChains" -v
```

Expected: **PASS** — all three new tests pass.

---

### Task 3: Migrate commands to the shared helper

**Files:**
- Modify: `cmd/wikismit/generate.go`
- Modify: `cmd/wikismit/plan.go`
- Modify: `cmd/wikismit/update.go`

- [ ] **Step 1: Migrate generate.go to use resolveClient**

Replace the inline nil-check + `llm.NewClient` in the `RunE` closure:

Before (lines 24-34 approximately):
```go
client := agentClientFactory()
if client == nil {
    var err error
    client, err = llm.NewClient(cfg.LLM)
    if err != nil {
        return err
    }
}
return runGenerate(cmd, cfg, client)
```

After:
```go
client, err := resolveClient(agentClientFactory, &cfg)
if err != nil {
    return err
}
return runGenerate(cmd, cfg, client)
```

Keep the `agentClientFactory` package-level variable for test seam compatibility.

- [ ] **Step 2: Migrate plan.go to use resolveClient**

Replace the inline nil-check in plan.go's `RunE` closure:

Before:
```go
client := plannerClientFactory()
if client == nil {
    client, err = llm.NewClient(cfg.LLM)
    if err != nil {
        return err
    }
}
```

After:
```go
client, err := resolveClient(plannerClientFactory, &cfg)
if err != nil {
    return err
}
```

Keep the `plannerClientFactory` variable.

- [ ] **Step 3: Migrate update.go to use resolveClientWithFallback**

Replace the two-tier fallback in update.go's `RunE`:

Before (lines 26-36 approximately):
```go
client := updateClientFactory()
if client == nil {
    client = agentClientFactory()
}
if client == nil {
    var err error
    client, err = llm.NewClient(cfg.LLM)
    if err != nil {
        return err
    }
}
```

After:
```go
client, err := resolveClientWithFallback(updateClientFactory, agentClientFactory, &cfg)
if err != nil {
    return err
}
```

Keep both `updateClientFactory` and `agentClientFactory` variables. The `agentClientFactory` import from generate.go is already visible since they're in the same package.

- [ ] **Step 4: Verify all command tests still pass**

Run:

```bash
go test ./cmd/wikismit -v
```

Expected: **PASS** — all existing tests still pass because:
- Factory variables are unchanged
- `resolveClient` produces the same behavior as the inline code
- Tests that override factories continue to work via the same variable reassignment pattern

---

### Task 4: Verify command regressions

**Files:**
- No file changes — verification only

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -count=1
```

Expected: **PASS** — zero failures across all 13+ packages.

- [ ] **Step 2: Verify no remaining inline client creation**

Search for any remaining `llm.NewClient(cfg.LLM)` calls outside of `helpers.go`:

```bash
grep -rn "llm.NewClient" cmd/wikismit/
```

Expected: **no matches** — all client creation now routes through the shared helper.

- [ ] **Step 3: Commit**

```bash
git add cmd/wikismit/helpers.go cmd/wikismit/generate.go cmd/wikismit/plan.go cmd/wikismit/update.go cmd/wikismit/main_test.go
git commit -m "refactor: consolidate command-side client creation into shared helper

Replace duplicated nil-check + llm.NewClient patterns in generate, plan,
and update commands with resolveClient and resolveClientWithFallback helpers
in cmd/wikismit/helpers.go.

Preserves all existing test-factory seams (agentClientFactory,
plannerClientFactory, updateClientFactory) and the update command's
two-tier fallback chain."
```
