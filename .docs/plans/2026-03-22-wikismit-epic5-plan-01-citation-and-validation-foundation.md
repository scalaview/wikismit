# Epic 5 Citation and Validation Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the deterministic Phase 5 citation injector, validation-report storage, cross-reference validator, and the real `wikismit validate` command.

**Architecture:** Start by locking the pure string/file-index transformation behavior for citations before adding any filesystem orchestration. Then add validation-report storage in `pkg/store`, implement the broken-link validator around those types, and only after that replace the CLI stub with real `validate` command wiring.

**Tech Stack:** Go, standard `testing`, `regexp`, `os`, `path/filepath`, existing `pkg/store`, existing Cobra command package.

---

### Task 1: Scaffold `internal/composer` with symbol-map and citation tests first

**Files:**
- Create: `internal/composer/citation.go`
- Create: `internal/composer/citation_test.go`

**Step 1: Write the failing symbol-map and citation tests**

Add:

```go
func TestBuildSymbolMapIncludesFunctionAndTypeRefs(t *testing.T) {}
func TestInjectCitationsReplacesExportedBacktickSymbols(t *testing.T) {}
```

Cover these behaviors:

- `buildSymbolMap` turns `store.FileIndex` entries into `path#Lline` refs
- exported backtick identifiers like `` `GenerateToken` `` become Markdown links

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestBuildSymbolMapIncludesFunctionAndTypeRefs|TestInjectCitationsReplacesExportedBacktickSymbols' -v
```

Expected: FAIL because the package and functions do not exist yet.

**Step 3: Write minimal implementation**

Implement in `internal/composer/citation.go`:

```go
func buildSymbolMap(idx store.FileIndex) map[string]string
func InjectCitations(content string, symbolMap map[string]string) string
```

Requirements:

- compile the citation regex once at package level
- include both function and type declarations in the map
- only match exported identifiers in backticks

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Lock citation edge cases before any file mutation helper

**Files:**
- Modify: `internal/composer/citation.go`
- Modify: `internal/composer/citation_test.go`

**Step 1: Write the failing edge-case tests**

Add:

```go
func TestInjectCitationsSkipsAlreadyLinkedAndUnknownNames(t *testing.T) {}
func TestInjectCitationsReplacesMultipleOccurrences(t *testing.T) {}
func TestInjectCitationsSkipsLowercaseIdentifiers(t *testing.T) {}
func TestBuildSymbolMapPrefersExportedSymbolForAmbiguousName(t *testing.T) {}
```

Cover these behaviors:

- already-linked names stay unchanged
- unknown names stay unchanged
- repeated occurrences all update
- lowercase identifiers do not match
- duplicate symbol names choose a canonical ref deterministically

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestInjectCitationsSkipsAlreadyLinkedAndUnknownNames|TestInjectCitationsReplacesMultipleOccurrences|TestInjectCitationsSkipsLowercaseIdentifiers|TestBuildSymbolMapPrefersExportedSymbolForAmbiguousName' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Finish the citation logic so it:

- avoids double-linking
- preserves unmatched text exactly
- handles multiple matches in one document
- prefers exported symbols for ambiguous names, then alphabetical file path order

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Add file-level citation processing after pure-string behavior is stable

**Files:**
- Modify: `internal/composer/citation.go`
- Modify: `internal/composer/citation_test.go`

**Step 1: Write the failing file processor test**

Add:

```go
func TestProcessFileOverwritesMarkdownWithInjectedCitations(t *testing.T) {}
```

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/composer -run TestProcessFileOverwritesMarkdownWithInjectedCitations -v
```

Expected: FAIL because `ProcessFile` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func ProcessFile(path string, symbolMap map[string]string) error
```

Requirements:

- read the existing file
- call `InjectCitations`
- overwrite the same file only when the read succeeds

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Add failing tests for validation-report persistence before store changes

**Files:**
- Modify: `pkg/store/artifacts.go`
- Modify: `pkg/store/index.go`
- Modify: `pkg/store/store_test.go`

**Step 1: Write the failing store test**

Add:

```go
func TestWriteValidationReportRoundTripsJSON(t *testing.T) {}
```

Assert that a written report file can be read back via generic JSON decode and retains `BrokenLinks`, `TotalLinks`, and `TotalFiles`.

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./pkg/store -run TestWriteValidationReportRoundTripsJSON -v
```

Expected: FAIL because the types and writer do not exist.

**Step 3: Write minimal implementation**

Add to `pkg/store`:

```go
type BrokenLink struct {
    SourceFile string    `json:"source_file"`
    LinkText   string    `json:"link_text"`
    LinkTarget string    `json:"link_target"`
    Line       int       `json:"line"`
}

type ValidationReport struct {
    GeneratedAt time.Time   `json:"generated_at"`
    BrokenLinks []BrokenLink `json:"broken_links"`
    TotalLinks  int          `json:"total_links"`
    TotalFiles  int          `json:"total_files"`
}

func WriteValidationReport(dir string, report ValidationReport) error
```

Write to `artifacts/validation_report.json` through the existing atomic JSON helper path.

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Add failing validator tests before wiring the CLI command

**Files:**
- Create: `internal/composer/validator.go`
- Create: `internal/composer/validator_test.go`

**Step 1: Write the failing validator tests**

Add:

```go
func TestValidateDocsReportsOnlyMissingInternalTargets(t *testing.T) {}
func TestValidateDocsSkipsExternalAndAnchorLinks(t *testing.T) {}
func TestValidateDocsHandlesEmptyDocsDirectory(t *testing.T) {}
```

Cover these behaviors:

- one missing relative Markdown link produces exactly one broken-link entry
- `https://...` and `#anchor` links are excluded
- empty `docs/` produces a zero-value report without error

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestValidateDocsReportsOnlyMissingInternalTargets|TestValidateDocsSkipsExternalAndAnchorLinks|TestValidateDocsHandlesEmptyDocsDirectory' -v
```

Expected: FAIL because `ValidateDocs` does not exist.

**Step 3: Write minimal implementation**

Implement in `internal/composer/validator.go`:

```go
func ValidateDocs(docsDir string) (store.ValidationReport, error)
```

Requirements:

- walk all `.md` files under `docsDir`
- extract Markdown links with a compiled regex
- resolve internal targets relative to the source file directory
- populate `store.ValidationReport`

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 6: Replace the `validate` command stub only after validator behavior is locked

**Files:**
- Modify: `cmd/wikismit/validate.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing CLI tests**

Add:

```go
func TestValidateCommandWritesValidationReportArtifact(t *testing.T) {}
func TestValidateCommandPrintsBrokenLinkSummaryAndExitsZero(t *testing.T) {}
```

Cover these behaviors:

- the command reads `cfg.OutputDir`
- it writes `validation_report.json` into `cfg.ArtifactsDir`
- it prints a summary line to stdout or stderr according to the command’s existing pattern
- it returns nil even when broken links are reported

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestValidateCommandWritesValidationReportArtifact|TestValidateCommandPrintsBrokenLinkSummaryAndExitsZero' -v
```

Expected: FAIL because the command still returns `not implemented`.

**Step 3: Write minimal implementation**

Replace the stub so `newValidateCmd()`:

- calls `composer.ValidateDocs(cfg.OutputDir)`
- writes the report via `store.WriteValidationReport(cfg.ArtifactsDir, report)`
- prints a stable summary such as `Validation complete: {n} broken links found in {totalFiles} files`
- returns `nil` even when `BrokenLinks` is non-empty

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 7: Verify the full citation + validation slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run composer-focused tests**

Run:

```bash
go test ./internal/composer -v
```

Expected: PASS.

**Step 2: Run store + CLI verification**

Run:

```bash
go test ./pkg/store ./cmd/wikismit -v
```

Expected: PASS.

**Step 3: Run broader regression check**

Run:

```bash
go test ./... 
```

Expected: PASS with no new failures.
