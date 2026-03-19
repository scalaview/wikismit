# wikismit — Epic 6: Incremental Update + CI Examples

**Status:** `todo`
**Depends on:** Epic 5
**Goal:** `wikismit update` re-runs only the pipeline phases needed for changed files, producing output identical to a full run for the changed modules. CI example files allow users to deploy docs to GitHub Pages with minimal setup.
**Spec refs:** §9 Incremental Update Mode, §16 CI integration examples

---

## S6.1 — Git diff parser

**Status:** `todo`

**Description:**
Implement `pkg/gitdiff/diff.go` using `go-git`. Given a repository path and optional base/head refs, return the set of modified file paths relative to the repo root. Support `--changed-files` to bypass git entirely.

**Acceptance criteria:**
- Given a repo with a known commit modifying `internal/auth/jwt.go` → returns `{"internal/auth/jwt.go"}`
- `--changed-files=a.go,b.go` → returns `{"a.go", "b.go"}` without reading git
- Clean repo (no diff) → empty set, no error
- All paths returned relative to repo root

**Files to create:**
```
pkg/gitdiff/diff.go
pkg/gitdiff/diff_test.go
```

### Subtasks

#### S6.1.1 — Add `go-git` dependency

- Add `github.com/go-git/go-git/v5` to `go.mod`
- Verify `go mod tidy` succeeds

#### S6.1.2 — Implement `DiffResult` type and `GetChangedFiles`

```go
type ChangeType string

const (
    ChangeModified ChangeType = "modified"
    ChangeAdded    ChangeType = "added"
    ChangeDeleted  ChangeType = "deleted"
    ChangeRenamed  ChangeType = "renamed"
)

type FileChange struct {
    Path    string     // new path (or old path for deleted)
    OldPath string     // only set for renamed files
    Type    ChangeType
}

func GetChangedFiles(repoPath, baseRef, headRef string) ([]FileChange, error)
```

- Open repo with `git.PlainOpen(repoPath)`
- Resolve `baseRef` and `headRef` to commit hashes
- Get the diff between the two trees: `base.Tree()`, `head.Tree()`, `baseTree.Diff(headTree)`
- Map each `diff.FilePatch` to a `FileChange` based on `patch.From()` and `patch.To()` states

#### S6.1.3 — Handle ref defaults

- Default `baseRef` to `"HEAD~1"` when not specified
- Default `headRef` to `"HEAD"` when not specified
- If `HEAD~1` does not exist (initial commit) → return all files as `ChangeAdded`
- Resolve ref strings to hashes using `repo.ResolveRevision`

#### S6.1.4 — Implement `--changed-files` bypass

- Implement `ParseChangedFiles(input string) []FileChange`:
  - Split on comma: `strings.Split(input, ",")`
  - Trim whitespace from each path
  - Return each as `FileChange{Path: path, Type: ChangeModified}`
- Used when `--changed-files` flag is provided; git is not opened at all

#### S6.1.5 — Git diff unit tests

- Use `go-git`'s in-memory repository for tests:
  - Create in-memory repo, add a file, commit → commit A
  - Modify the file, commit → commit B
  - Test: `GetChangedFiles(repo, "A", "B")` → returns the modified file path
- Test: rename `old.go` → `new.go` → `FileChange{OldPath: "old.go", Path: "new.go", Type: ChangeRenamed}`
- Test: add `newfile.go` → `FileChange{Path: "newfile.go", Type: ChangeAdded}`
- Test: delete `file.go` → `FileChange{Path: "file.go", Type: ChangeDeleted}`
- Test: `ParseChangedFiles("a.go,b.go")` → two `FileChange` entries with `ChangeModified`
- Test: `ParseChangedFiles("")` → empty slice, no error

---

## S6.2 — Affected module computation

**Status:** `todo`

**Description:**
Given the set of changed files and `dep_graph.json`, compute which modules must be re-run: direct owners of changed files plus all upstream modules that transitively depend on them.

**Acceptance criteria:**
- Changing `pkg/logger/logger.go` → `logger` (shared) + all modules depending on it are marked affected
- Changing a file in a leaf module → only that module is affected
- Affected set is a subset of all modules in `nav_plan.json`

**Files to create:**
```
internal/analyzer/affected.go
internal/analyzer/affected_test.go
```

### Subtasks

#### S6.2.1 — Map changed files to owning modules

- Implement `owningModules(changedFiles []string, plan *store.NavPlan) []string`:
  - For each changed file path, find the module in `plan.Modules` whose `Files` list contains it
  - Return deduplicated list of module IDs

#### S6.2.2 — Build reverse dependency graph

- Implement `buildReverseGraph(graph store.DepGraph) store.DepGraph`:
  - For each edge `A → B`, add edge `B → A` in the reverse graph
  - Used to find "who depends on this module"

#### S6.2.3 — Implement transitive upstream propagation

- Implement `ComputeAffected(changedFiles []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module`:
  1. Find directly owning modules of changed files
  2. Build file-level reverse graph
  3. BFS from each owning module's files through the reverse graph to find all upstream dependents
  4. Map affected file paths back to module IDs
  5. Return the union of direct + transitive affected modules, deduplicated

#### S6.2.4 — Affected module unit tests

- Setup: use `testdata/sample_repo/` nav_plan and dep_graph
- Test: `internal/auth/jwt.go` changed → `auth` module affected only (no other module imports auth)
- Test: `pkg/logger/logger.go` changed → `logger` (shared) + `auth`, `api`, `db` all affected
- Test: `pkg/errors/errors.go` changed → `errors` + `auth`, `db` affected (but not `api` if it doesn't import errors)
- Test: non-existent file in changed set → ignored, no panic

---

## S6.3 — Incremental pipeline orchestration

**Status:** `todo`

**Description:**
Implement the `wikismit update` command. Load existing artifacts, compute affected modules, re-run Phase 3 for affected shared modules and Phase 4 for affected non-shared modules, then run Phase 5 in full. Fall back to full generate if no artifacts exist.

**Acceptance criteria:**
- Only 1 module changed in an 8-module repo → exactly 1 LLM call in Phase 4
- Affected shared modules re-run in Phase 3 before their dependents in Phase 4
- Phase 5 always runs in full
- Falls back to `wikismit generate` with a warning when no artifacts exist

**Files to create:**
```
cmd/wikismit/update.go
internal/pipeline/incremental.go
```

### Subtasks

#### S6.3.1 — Implement `RunIncremental` orchestration

```go
func RunIncremental(ctx context.Context, cfg *config.Config, llm llm.Client) error {
    // 1. Check artifacts exist
    idx, err := store.ReadFileIndex(cfg.ArtifactsDir)
    if errors.Is(err, store.ErrArtifactNotFound) {
        log.Warn("No existing artifacts found, falling back to full generate")
        return RunGenerate(ctx, cfg, llm)
    }

    // 2. Get changed files
    changes, err := gitdiff.GetChangedFiles(cfg.RepoPath, baseRef, headRef)

    // 3. Re-parse changed files only (update FileIndex in place)
    idx, err = reanalyzeChanged(changes, idx, cfg)

    // 4. Rebuild dep graph (cheap, always full)
    graph := analyzer.BuildDepGraph(idx)

    // 5. Load nav_plan (do NOT re-run Phase 2)
    plan, err := store.ReadNavPlan(cfg.ArtifactsDir)

    // 6. Compute affected modules
    affected := analyzer.ComputeAffected(changes, plan, graph)

    // 7. Re-run Phase 3 for affected shared modules (serial, topo order)
    sharedCtx, err := preprocessor.RunPreprocessorFor(ctx, affected, plan, idx, graph, cfg, llm)

    // 8. Re-run Phase 4 for affected non-shared modules (parallel)
    err = agent.RunFor(ctx, affected, plan, idx, sharedCtx, cfg, llm)

    // 9. Phase 5 always runs in full
    return composer.RunComposer(cfg, plan, idx, graph)
}
```

#### S6.3.2 — Implement `reanalyzeChanged`

- For each `FileChange`:
  - `ChangeModified` or `ChangeAdded`: re-parse the file, update `idx[path]`
  - `ChangeDeleted`: delete `idx[path]`
  - `ChangeRenamed`: delete `idx[oldPath]`, parse new file, set `idx[newPath]`
- Write updated `file_index.json` back to store
- Return updated `FileIndex`

#### S6.3.3 — Implement `RunPreprocessorFor` (partial preprocessor)

- Filter `affected` to only shared modules (`owner == "shared_preprocessor"`)
- Re-run topological sort on the shared subset
- For unaffected shared modules: load existing summary from `shared_context.json`
- For affected shared modules: re-run LLM call and update their entry in `SharedContext`
- Write updated `shared_context.json`

#### S6.3.4 — Implement `RunFor` (partial agent fan-out)

- Filter `affected` to only non-shared modules (`owner == "agent"`)
- Run Phase 4 scheduler on this subset only
- Unaffected modules' `module_docs/*.md` files are left untouched

#### S6.3.5 — Wire `wikismit update` command

- In `cmd/wikismit/update.go`:
  - Add `--base-ref` flag (default: `HEAD~1`)
  - Add `--head-ref` flag (default: `HEAD`)
  - Add `--changed-files` flag (bypasses git)
  - Call `RunIncremental`

#### S6.3.6 — Incremental orchestration unit tests

- Test: 8-module repo, 1 file changed → `MockClient.CallCount() == 1` (only 1 Phase 4 agent call)
- Test: shared module file changed → Phase 3 re-runs for that shared module + Phase 4 re-runs for all its dependents
- Test: no artifacts exist → falls back to `RunGenerate`, logs warning
- Test: deleted file → removed from `file_index.json`, corresponding module re-runs

---

## S6.4 — CI example files

**Status:** `todo`

**Description:**
Write the example GitHub Actions workflow files and README in `examples/github/`. Workflows use the final CLI. README is self-contained for a new user setting up GitHub Pages.

**Acceptance criteria:**
- `docs-full.yml`: full generation on push to `main`, deploy to GitHub Pages
- `docs-incremental.yml`: two-job pattern with `artifacts/` cache, runs `wikismit update`
- `README.md`: covers required secrets, enabling Pages, Cloudflare Pages alternative
- Both YAML files pass syntax validation

**Files to create:**
```
examples/github/docs-full.yml
examples/github/docs-incremental.yml
examples/github/README.md
```

### Subtasks

#### S6.4.1 — Write `docs-full.yml`

```yaml
name: Generate and deploy docs

on:
  push:
    branches: [main]

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  docs:
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Install wikismit
        run: |
          curl -sSL https://github.com/scalaview/wikismit/releases/latest/download/wikismit-linux-amd64 \
            -o /usr/local/bin/wikismit && chmod +x /usr/local/bin/wikismit

      - name: Generate docs
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: wikismit generate --repo=. --output=./docs

      - uses: actions/setup-node@v4
        with:
          node-version: 20

      - name: Build VitePress site
        run: wikismit build

      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs/.vitepress/dist

      - uses: actions/deploy-pages@v4
        id: deployment
```

#### S6.4.2 — Write `docs-incremental.yml`

```yaml
name: Update docs (incremental)

on:
  push:
    branches: [main]

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Restore artifacts cache
        uses: actions/cache@v4
        with:
          path: artifacts/
          key: wikismit-artifacts-${{ github.sha }}
          restore-keys: wikismit-artifacts-

      - name: Install wikismit
        run: |
          curl -sSL https://github.com/scalaview/wikismit/releases/latest/download/wikismit-linux-amd64 \
            -o /usr/local/bin/wikismit && chmod +x /usr/local/bin/wikismit

      - name: Update docs
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: wikismit update --repo=.

      - uses: actions/upload-artifact@v4
        with:
          name: generated-docs
          path: docs/

  deploy:
    needs: generate
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    permissions:
      pages: write
      id-token: write
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: generated-docs
          path: docs/

      - uses: actions/setup-node@v4
        with:
          node-version: 20

      - name: Build VitePress site
        run: |
          npm install -D vitepress
          npx vitepress build docs

      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs/.vitepress/dist

      - uses: actions/deploy-pages@v4
        id: deployment
```

#### S6.4.3 — Write `examples/github/README.md`

Structure:
1. **Prerequisites** — GitHub repo with Pages enabled, `OPENAI_API_KEY` secret added
2. **Quick start (full generation)** — copy `docs-full.yml` to `.github/workflows/`, push to main
3. **Incremental updates** — copy `docs-incremental.yml` instead; explain the cache key strategy
4. **Enable GitHub Pages** — Settings → Pages → Source: GitHub Actions
5. **Cloudflare Pages alternative** — replace the last 2 steps with `cloudflare/pages-action@v1`, list the required secrets (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `projectName`)
6. **Using a different LLM provider** — set `base_url` in `config.yaml` (Anthropic proxy, Ollama)
7. **Troubleshooting** — common issues: missing secret, Node.js not found, no artifacts cache on first run

#### S6.4.4 — Validate workflow YAML syntax

- Add a `Makefile` target `lint-ci` that runs `actionlint` on both workflow files
- Alternatively, document in `README.md` that the workflows can be validated with `actionlint` before committing
- Both files must pass without warnings
