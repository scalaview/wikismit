# Epic 6 CI Example Files Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship copyable GitHub Actions examples and setup docs that match the real `wikismit generate`, `wikismit update`, and `wikismit build` behavior.

**Architecture:** First lock the example workflow text against the exact CLI commands the repo now supports, keeping the YAML intentionally simple and opt-in. Then add the setup README with focused GitHub Pages and Cloudflare Pages guidance, and finish with lightweight workflow validation instructions that fit the current repo layout without inventing a new build system.

**Tech Stack:** YAML, Markdown, existing CLI behavior, GitHub Actions conventions, VitePress output contract from Epic 5.

---

### Task 1: Write the full-generation workflow before documenting it

**Files:**
- Create: `examples/github/docs-full.yml`

**Step 1: Add the workflow file from a failing file-existence check**

Create a small table-driven test or focused file assertion if you want automation first, otherwise use a manual RED check that confirms `examples/github/docs-full.yml` does not exist yet.

**Step 2: Write the workflow**

Requirements:

- trigger on push to `main`
- checkout with `fetch-depth: 0`
- install the released `wikismit` binary
- run `wikismit generate`
- run `wikismit build`
- upload `docs/.vitepress/dist`
- deploy with `actions/deploy-pages`

Keep the commands aligned with the current build behavior in `cmd/wikismit/build.go`.

**Step 3: Verify the file contents manually**

Check that the workflow mentions:

- `OPENAI_API_KEY`
- Node 20 setup
- `docs/.vitepress/dist`

Expected: all are present.

### Task 2: Write the incremental workflow with artifact/cache behavior

**Files:**
- Create: `examples/github/docs-incremental.yml`

**Step 1: Confirm RED**

Verify the file does not exist yet.

**Step 2: Write the workflow**

Requirements:

- two-job shape (`generate` + `deploy`)
- restore `artifacts/` cache before `wikismit update`
- run `wikismit update --repo=.`
- upload generated docs between jobs
- build and deploy the resulting VitePress output

Do not assume extra repo tooling beyond GitHub Actions, npm, and the shipped CLI.

**Step 3: Verify the file contents manually**

Check that the workflow mentions:

- `actions/cache@v4`
- `wikismit update`
- `actions/upload-artifact@v4`
- `actions/deploy-pages@v4`

Expected: all are present.

### Task 3: Write the setup README for GitHub Pages and Cloudflare Pages

**Files:**
- Create: `examples/github/README.md`

**Step 1: Confirm RED**

Verify the README does not exist yet.

**Step 2: Write the README**

Required sections:

1. prerequisites (`OPENAI_API_KEY`, Pages enabled, repo checkout expectations)
2. quick start for `docs-full.yml`
3. incremental flow for `docs-incremental.yml`
4. enabling GitHub Pages from GitHub Actions
5. Cloudflare Pages alternative with required secrets
6. alternate OpenAI-compatible providers via `base_url`
7. troubleshooting notes for missing secrets, Node, and first-run cache behavior

Keep the README self-contained and aligned with current repo commands.

**Step 3: Review for drift against the actual CLI**

Confirm the README references only commands that exist now:

- `wikismit generate`
- `wikismit update`
- `wikismit build`

Expected: no stale or imaginary commands appear.

### Task 4: Validate workflow syntax without inventing a new repo toolchain

**Files:**
- Modify: `examples/github/README.md`
- Modify only if needed: `examples/github/docs-full.yml`
- Modify only if needed: `examples/github/docs-incremental.yml`

**Step 1: Add lightweight validation guidance**

Document `actionlint` usage in the README, for example:

```bash
actionlint examples/github/docs-full.yml examples/github/docs-incremental.yml
```

Do not add a `Makefile`; this worktree does not have one.

**Step 2: Run validation if `actionlint` is available**

Run:

```bash
actionlint examples/github/docs-full.yml examples/github/docs-incremental.yml
```

Expected: PASS if the tool is installed. If it is unavailable, note that explicitly and verify the YAML structure manually.

**Step 3: Fix only syntax/documentation issues found**

Keep any fix minimal and limited to the example files.

### Task 5: Verify the example-files slice

**Files:**
- Modify only if validation fixes are required

**Step 1: Read all example files end to end**

Confirm the three files are present and internally consistent.

**Step 2: Run targeted repo regression check**

Run:

```bash
go test ./cmd/wikismit ./internal/composer -v
```

Expected: PASS. The example docs should not disturb the Go build/test surface.

**Step 3: Record validation status**

If `actionlint` ran, note that it passed. If not available, note that the workflows were manually checked against the accepted GitHub Actions structure and the current CLI contract.
