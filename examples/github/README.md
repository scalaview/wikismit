# GitHub Actions examples for wikismit

These examples show two supported deployment flows for a repository that already contains a working `wikismit` configuration:

- `docs-full.yml` runs a full `wikismit generate` on every push to `main`
- `docs-incremental.yml` restores the `artifacts/` cache and runs `wikismit update`

## Prerequisites

Before using either workflow:

1. Add a working `config.yaml` (or adapt your repo so the default path is valid).
2. Add the `OPENAI_API_KEY` repository secret.
3. Enable GitHub Pages for the repository.
4. Make sure the repository can fetch full git history in CI (`actions/checkout` uses `fetch-depth: 0`).

## Quick start: full generation

If you want the simplest deployment path:

1. Copy `examples/github/docs-full.yml` to `.github/workflows/docs.yml`
2. Commit the workflow
3. Push to `main`

The workflow will:

- install the released `wikismit` binary
- run `wikismit generate --repo=. --output=./docs --artifacts=./artifacts`
- run `wikismit build --output=./docs`
- upload `docs/.vitepress/dist`
- deploy the static site to GitHub Pages

## Incremental updates

If you already persist `artifacts/` between runs and want cheaper updates:

1. Copy `examples/github/docs-incremental.yml` to `.github/workflows/docs.yml`
2. Commit the workflow
3. Push to `main`

This flow restores the `artifacts/` cache, runs `wikismit update --repo=. --output=./docs --artifacts=./artifacts`, then uploads the generated docs to a second deploy job.

### Cache notes

- The example uses `wikismit-artifacts-${{ github.sha }}` as the primary cache key.
- `restore-keys: wikismit-artifacts-` allows a previous cache to seed the first incremental run on a new commit.
- On the first run, a cache miss is expected; `wikismit update` falls back to a full generate when no existing artifacts are found.

## Enable GitHub Pages

In GitHub:

1. Open **Settings → Pages**
2. Set **Source** to **GitHub Actions**

After that, `actions/deploy-pages` will publish `docs/.vitepress/dist` directly.

## Cloudflare Pages alternative

If you prefer Cloudflare Pages, keep the generation steps and replace the final GitHub Pages deploy steps with Cloudflare deployment.

Typical required secrets:

- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`
- your Cloudflare Pages `projectName`

You can use `cloudflare/pages-action@v1` after the `vitepress build` step and point it at `docs/.vitepress/dist`.

## Using a different LLM provider

The workflows only provide the API key environment variable. Provider switching still happens in `config.yaml`.

Examples:

- OpenAI: default `base_url`
- Anthropic-compatible proxy: set `base_url` to the proxy endpoint
- Ollama: set `base_url: http://localhost:11434/v1`

No workflow change is required as long as the runtime environment can reach that endpoint.

## Troubleshooting

### Missing secret

If the workflow fails with an API key or env-var error, confirm `OPENAI_API_KEY` exists in repository secrets.

### Node.js not found

`wikismit build` requires Node.js 20+. Both example workflows explicitly install Node 20 before the build step.

### No artifacts cache yet

That is normal on the first incremental run. The example `docs-incremental.yml` still succeeds because `wikismit update` falls back to a full generate when artifacts are missing.

### Config path differences

If your repository stores config somewhere else, add `--config path/to/config.yaml` to the `wikismit generate`, `wikismit update`, and `wikismit build` commands in the workflow.

## Workflow validation

If `actionlint` is installed locally, validate the examples with:

```bash
actionlint examples/github/docs-full.yml examples/github/docs-incremental.yml
```

This repository does not add a `Makefile` just for workflow validation; keep the check lightweight.
