# wikismit — Epic 5: Phase 5 — Doc Composer + VitePress Output

**Status:** `todo`  
**Depends on:** Epic 4  
**Goal:** Given `artifacts/module_docs/`, produce a complete `docs/` directory with injected citations, validated cross-references, TOC, and a `docs/.vitepress/config.ts` ready for `vitepress build`.  
**Spec refs:** §4 Phase 5, §16 Documentation Deployment

---

## S5.1 — Citation injector

**Status:** `todo`

**Description:**  
Implement `internal/composer/citation.go`. Scan each `module_docs/*.md` for function and type name references inside backticks. Look up each name in `file_index.json`. If a `file:line` link is absent for that name, inject it as a Markdown link. Names not found in the index are left unchanged.

**Acceptance criteria:**
- `` `GenerateToken` `` → `[GenerateToken](internal/auth/jwt.go#L24)` when found in `file_index.json`
- Already-linked names are not double-linked
- Names absent from `file_index.json` are left as-is, no error
- Table-driven tests cover all four cases: found, already linked, not in index, appears multiple times

**Files to create:**
```
internal/composer/citation.go
internal/composer/citation_test.go
```

### Subtasks

#### S5.1.1 — Build symbol lookup map from `FileIndex`

- Implement `buildSymbolMap(idx store.FileIndex) map[string]string`:
  - Key: symbol name (e.g. `"GenerateToken"`)
  - Value: `"{path}#L{LineStart}"` (e.g. `"internal/auth/jwt.go#L24"`)
  - For functions: use `FunctionDecl.Name` → `"{filePath}#L{LineStart}"`
  - For types: use `TypeDecl.Name` → `"{filePath}#L{LineStart}"`
  - If the same name appears in multiple files, prefer exported symbols; for ties, use the first file alphabetically and log `WARN "ambiguous symbol: {name} found in {files}"`

#### S5.1.2 — Implement citation injection regex

- Pattern to match backtick-wrapped identifiers that are NOT already Markdown links:
  ```
  (?<!]\()   # not preceded by ]( (already linked)
  `([A-Z][A-Za-z0-9]+)`   # exported identifier in backticks
  ```
- Use `regexp.MustCompile` and compile once at package level
- For each match: if name found in symbol map → replace `` `Name` `` with `[Name](path#LN)`
- If not found → leave unchanged

#### S5.1.3 — Implement `InjectCitations(content string, symbolMap map[string]string) string`

- Apply the regex substitution to the full document content
- Track injected count for logging: `DEBUG "citation: injected {n} links in {filename}"`
- Return the modified content string (no in-place file mutation in this function)

#### S5.1.4 — File-level citation processor

- Implement `ProcessFile(path string, symbolMap map[string]string) error`:
  - Read file content
  - Call `InjectCitations`
  - Write result back to the same path (overwrite)

#### S5.1.5 — Citation unit tests

- Test table:
  | Input | Symbol map | Expected output |
  |---|---|---|
  | `` `GenerateToken` `` | `GenerateToken → auth/jwt.go#L24` | `[GenerateToken](auth/jwt.go#L24)` |
  | `[GenerateToken](auth/jwt.go#L24)` | same | unchanged (already linked) |
  | `` `unknownFunc` `` | empty | unchanged |
  | `` `GenerateToken` appears twice `` | same | both occurrences replaced |
- Test: all-lowercase identifier `` `myHelper` `` is not matched (only exported symbols)
- Test: symbol in both `Functions` and `Types` with same name → `WARN` logged, one canonical ref chosen

---

## S5.2 — Cross-reference validator

**Status:** `todo`

**Description:**  
Implement `internal/composer/validator.go`. Scan all output Markdown files for internal links. Verify each target file exists in the `docs/` output directory. Collect broken links into a `ValidationReport`. Wire `wikismit validate` subcommand.

**Acceptance criteria:**
- Link to `../shared/logger.md` that doesn't exist → appears in `ValidationReport`
- Valid link → no warning
- `wikismit validate` exits 0 even with warnings (non-blocking in v1)
- Report written to `artifacts/validation_report.json`

**Files to create:**
```
internal/composer/validator.go
internal/composer/validator_test.go
```

### Subtasks

#### S5.2.1 — Define `ValidationReport` type

```go
type BrokenLink struct {
    SourceFile string
    LinkText   string
    LinkTarget string
    Line       int
}

type ValidationReport struct {
    GeneratedAt    time.Time
    BrokenLinks    []BrokenLink
    TotalLinks     int
    TotalFiles     int
}
```

- Add to `pkg/store`: `WriteValidationReport(dir string, r ValidationReport) error`

#### S5.2.2 — Implement link extraction regex

- Match Markdown links: `\[([^\]]+)\]\(([^)]+)\)`
- Filter to internal links only: target does NOT start with `http://` or `https://`
- Exclude anchor-only links: target starts with `#`
- For each internal link, resolve the target path relative to the source file's directory

#### S5.2.3 — Implement `ValidateDocs(docsDir string) (ValidationReport, error)`

- Walk all `.md` files in `docsDir`
- For each file, extract all internal links (S5.2.2)
- For each link target, check `os.Stat(resolvedPath)`:
  - File exists → increment `TotalLinks`
  - File missing → append `BrokenLink` to report
- Return completed `ValidationReport`

#### S5.2.4 — Wire `wikismit validate` command

- In `cmd/wikismit/validate.go`:
  1. Call `ValidateDocs(cfg.OutputDir)`
  2. Write report via `store.WriteValidationReport`
  3. Print summary: `"Validation complete: {n} broken links found in {totalFiles} files"`
  4. Exit 0 regardless of broken link count (v1)

#### S5.2.5 — Validator unit tests

- Setup: write a temp `docs/` with two files, one linking to a file that exists and one to a file that does not
- Test: `ValidateDocs` returns `BrokenLinks` with exactly one entry for the missing target
- Test: `BrokenLink.SourceFile`, `LinkText`, `LinkTarget`, `Line` are all correctly populated
- Test: external links (`https://...`) are not included in broken link checks
- Test: anchor links (`#section`) are not included in broken link checks
- Test: empty `docs/` directory → report with 0 broken links, no error

---

## S5.3 — Markdown renderer and TOC generation

**Status:** `todo`

**Description:**  
Implement `internal/composer/renderer.go`. Assemble the final `docs/` directory from `artifacts/module_docs/`. Inject a TOC at the top of each file. Write `docs/index.md` (landing page with module tree from `nav_plan.json`). Run citation injection and cross-reference validation as part of this step.

**Acceptance criteria:**
- `docs/` contains `index.md`, `modules/*.md`, `shared/*.md` after Phase 5
- Every output file has a `## Contents` section with anchor links at the top
- `docs/index.md` lists all modules and shared modules with links, ordered by dependency depth
- File count and output dir logged at `INFO`

**Files to create:**
```
internal/composer/renderer.go
internal/composer/renderer_test.go
```

### Subtasks

#### S5.3.1 — Implement `GenerateTOC(content string) string`

- Parse all H2 (`## `) and H3 (`### `) headings from the Markdown content
- For each heading, generate a GitHub-compatible anchor: lowercase, replace spaces with `-`, strip non-alphanumeric characters
- Build a TOC block:
  ```markdown
  ## Contents

  - [Overview](#overview)
  - [Key Types](#key-types)
    - [Claims](#claims)
  - [Key Functions](#key-functions)
  ```
- Insert the TOC block after the first H1 heading (or at the top if no H1 present)

#### S5.3.2 — Implement `CopyModuleDocs(artifactsDir, docsDir string, plan *store.NavPlan, symbolMap map[string]string) error`

- For each non-shared module in `plan.Modules`:
  - Read `{artifactsDir}/module_docs/{moduleID}.md`
  - Call `InjectCitations(content, symbolMap)`
  - Call `GenerateTOC(content)`
  - Write to `{docsDir}/modules/{moduleID}.md` (create parent dirs as needed)
- For each shared module in `plan.Modules`:
  - Read `{artifactsDir}/module_docs/{moduleID}.md`
  - Same citation + TOC processing
  - Write to `{docsDir}/shared/{moduleID}.md`

#### S5.3.3 — Generate `docs/index.md`

- Implement `GenerateIndexPage(plan *store.NavPlan, graph store.DepGraph) string`:
  - Compute depth of each module in the dep graph (BFS from roots)
  - Sort modules by depth (shallowest first = leaf modules first)
  - Write:
    ```markdown
    # Project Documentation

    Auto-generated by wikismit.

    ## Modules

    | Module | Description |
    |--------|-------------|
    | [auth](modules/auth.md) | — |
    | [api](modules/api.md) | — |

    ## Shared Modules

    | Module | Used by |
    |--------|---------|
    | [logger](shared/logger.md) | auth, api, db |
    ```
  - "Used by" column from `module.ReferencedBy`

#### S5.3.4 — Implement `RunComposer(cfg *config.Config, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) error`

- Build symbol map: `buildSymbolMap(idx)`
- Ensure `docs/modules/` and `docs/shared/` directories exist
- Call `CopyModuleDocs`
- Write `docs/index.md` via `GenerateIndexPage`
- Call `ValidateDocs(cfg.OutputDir)` → write `artifacts/validation_report.json`
- Log `INFO "Phase 5 complete: {n} files written to {outputDir}"`

#### S5.3.5 — Wire Phase 5 into `generate` command

- In `cmd/wikismit/generate.go`, after Phase 4:
  - Load `nav_plan.json`, `file_index.json`, `dep_graph.json` from store
  - Call `RunComposer`

#### S5.3.6 — Renderer unit tests

- Test: `GenerateTOC` on a file with H2 + H3 headings → TOC block contains correct anchors
- Test: `GenerateTOC` on a file with no headings → content returned unchanged, no TOC injected
- Test: `CopyModuleDocs` on `testdata/sample_repo/` artifacts → `docs/modules/` and `docs/shared/` populated
- Test: `GenerateIndexPage` → `docs/index.md` contains links to all modules with correct relative paths
- Test: anchor generation for `"Key Functions"` → `#key-functions`
- Test: anchor generation for `"HTTP Handler"` → `#http-handler`

---

## S5.4 — VitePress config generator

**Status:** `todo`

**Description:**  
Implement `internal/composer/vitepress.go`. Generate `docs/.vitepress/config.ts` from `nav_plan.json` and site config fields. Sidebar groups modules by "Modules" and "Shared". Sidebar order follows dependency depth.

**Acceptance criteria:**
- Generated `config.ts` is valid TypeScript consumable by `vitepress build` without modification
- Sidebar entries match actual output file paths
- `site.title`, `site.repo_url` from `config.yaml` are reflected in `config.ts`
- Sensible defaults when `site.*` fields are absent

**Files to create:**
```
internal/composer/vitepress.go
internal/composer/vitepress_test.go
```

### Subtasks

#### S5.4.1 — Define VitePress config template

- Implement using Go `text/template`, not string concatenation:
  ```typescript
  import { defineConfig } from 'vitepress'

  export default defineConfig({
    title: '{{.Title}}',
    description: 'Auto-generated by wikismit',
    {{- if .RepoURL}}
    themeConfig: {
      editLink: {
        pattern: '{{.RepoURL}}/edit/main/docs/:path',
        text: 'Edit this page'
      },
    {{- else}}
    themeConfig: {
    {{- end}}
      search: { provider: 'local' },
      {{- if .Logo}}
      logo: '/logo.png',
      {{- end}}
      sidebar: [
        {{- range .SidebarGroups}}
        {
          text: '{{.GroupName}}',
          items: [
            {{- range .Items}}
            { text: '{{.Label}}', link: '{{.Link}}' },
            {{- end}}
          ]
        },
        {{- end}}
      ]
    }
  })
  ```

#### S5.4.2 — Implement `GenerateVitePressConfig(plan *store.NavPlan, graph store.DepGraph, cfg *config.Config) string`

- Determine `Title`: `cfg.Site.Title` if set, else repo directory name from `cfg.RepoPath`
- Compute module depth order (reuse from renderer S5.3.3)
- Build sidebar groups:
  - Group 1: `"Modules"` — non-shared modules sorted by depth
  - Group 2: `"Shared"` — shared modules sorted by depth
- Each item: `Label = module.ID`, `Link = "/modules/{id}"` or `"/shared/{id}"`

#### S5.4.3 — Handle site logo

- If `cfg.Site.Logo` is set:
  - Copy the file at that path to `docs/public/logo.png`
  - Include `logo: '/logo.png'` in the config
- If not set: omit the logo field entirely

#### S5.4.4 — Wire config generation into `RunComposer`

- After writing all Markdown files, call `GenerateVitePressConfig`
- Write output to `docs/.vitepress/config.ts` (create `.vitepress/` dir if needed)
- Log `INFO "VitePress config written to docs/.vitepress/config.ts"`

#### S5.4.5 — Wire `wikismit build` command

- In `cmd/wikismit/build.go`:
  1. Verify `docs/.vitepress/config.ts` exists; if not, error with `"run wikismit generate first"`
  2. Check `node` is available in PATH; if not, error with `"Node.js 20+ is required for wikismit build"`
  3. Run `npm install -D vitepress` in the `docs/` directory (only if `node_modules/` absent)
  4. Run `npx vitepress build docs`
  5. Log `INFO "Build complete: docs/.vitepress/dist/"` on success

#### S5.4.6 — VitePress config unit tests

- Test: `nav_plan.json` with 3 non-shared + 2 shared modules → config contains both sidebar groups with correct item counts
- Test: `cfg.Site.Title = "MyProject"` → config title is `"MyProject"`
- Test: `cfg.Site.Title` absent → config title is repo directory name
- Test: `cfg.Site.RepoURL` set → `editLink.pattern` appears in config
- Test: `cfg.Site.RepoURL` absent → no `editLink` section in config
- Test: generated config string parses as valid text (balanced braces, no template artifacts like `{{` in output)
