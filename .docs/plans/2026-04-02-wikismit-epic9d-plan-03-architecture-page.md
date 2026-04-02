# Plan 03 — Generate Architecture Overview Page

**Epic ref:** S9D.3
**Depends on:** Plan 01

---

## Goal

Generate `architecture.md` with purpose, layers, data flow, and Mermaid dependency graph. Add to VitePress sidebar as first item.

---

## Implementation steps

### Step 1: Add GenerateArchitecturePage function

**File:** `internal/composer/renderer.go`

```go
func GenerateArchitecturePage(plan *store.NavPlan, graph store.DepGraph) string {
    // 1. Title + Purpose (from ArchSummary)
    // 2. Layers list (if present)
    // 3. Data Flow paragraph (if present)
    // 4. Mermaid graph TD from module dependencies
    //    - Build module-level graph (reuse buildModuleGraph)
    //    - Render as: graph TD\n  api --> auth\n  auth --> db\n
}
```

Graceful degradation: if `ArchSummary` is nil, output heading + Mermaid graph only. If `ArchSummary` exists but fields are empty, skip those sections.

### Step 2: Call GenerateArchitecturePage in RunComposer

**File:** `internal/composer/renderer.go` — `RunComposer` function

After writing `index.md`, write `architecture.md`:

```go
archPage := GenerateArchitecturePage(plan, graph)
if err := os.WriteFile(filepath.Join(cfg.OutputDir, "architecture.md"), []byte(archPage), 0o644); err != nil {
    return err
}
```

### Step 3: Add Architecture to VitePress sidebar

**File:** `internal/composer/vitepress.go`

In `GenerateVitePressConfig`, add Architecture as first sidebar group item:

```go
sidebar: [
  { text: 'Architecture', link: '/architecture.md' },
  // existing Modules and Shared groups
]
```

Update the template to include a fixed Architecture link before the Modules group.

### Step 4: Add tests

**File:** `internal/composer/renderer_test.go`

- Test: `GenerateArchitecturePage` includes purpose when ArchSummary present
- Test: Mermaid graph contains correct module dependency edges
- Test: graceful output when ArchSummary is nil (just Mermaid graph + heading)
- Test: VitePress config includes Architecture link

---

## Verification

```bash
go test ./internal/composer/ -v
```
