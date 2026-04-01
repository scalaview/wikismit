# Epic 9A.3 Fix `BrokenLink.Line` Always Zero

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hardcoded `Line: 0` in `ValidateDocs` with a computed 1-based line number derived from the byte offset of each regex match, so broken link reports show where links actually appear in the source file.

**Architecture:** The `FindAllStringSubmatchIndex` call already returns byte offsets. Convert the start byte offset of each match to a line number by counting `\n` characters before that position. Use `bytes.Count(content[:match[0]], []byte("\n")) + 1`. Keep the existing regex, link extraction, and validation report structure unchanged.

**Tech Stack:** Go, `bytes`, `regexp`, `testing`, existing `internal/composer` validator.

---

### Task 1: Add failing tests for line number calculation

**Files:**
- Modify: `internal/composer/validator_test.go`
- Test: `internal/composer/validator_test.go`

- [ ] **Step 1: Add a test for broken links on various lines**

Add `TestValidateDocsReportsCorrectLineNumbers` after the existing tests. Create a multi-line markdown file with a valid link on line 1 and a broken link on line 3:

```go
func TestValidateDocsReportsCorrectLineNumbers(t *testing.T) {
	docsDir := t.TempDir()

	// Line 1: heading
	// Line 2: blank
	// Line 3: broken link
	// Line 4: valid link to existing file
	content := "# Auth\n\nSee [Missing](../shared/missing.md).\n[Exists](existing.md)\n"
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(index.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "existing.md"), []byte("# Exists\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(existing.md) error = %v", err)
	}

	report, err := ValidateDocs(docsDir)
	if err != nil {
		t.Fatalf("ValidateDocs() error = %v", err)
	}

	if len(report.BrokenLinks) != 1 {
		t.Fatalf("len(BrokenLinks) = %d, want 1", len(report.BrokenLinks))
	}

	// The broken link "[Missing](../shared/missing.md)" is on line 3 (1-based)
	if report.BrokenLinks[0].Line != 3 {
		t.Fatalf("Line = %d, want 3 (broken link is on the third line of the file)", report.BrokenLinks[0].Line)
	}
	if report.BrokenLinks[0].LinkTarget != "../shared/missing.md" {
		t.Fatalf("LinkTarget = %q, want %q", report.BrokenLinks[0].LinkTarget, "../shared/missing.md")
	}
}
```

- [ ] **Step 2: Add a test for broken link on first line**

```go
func TestValidateDocsReportsLineOneForBrokenLinkOnFirstLine(t *testing.T) {
	docsDir := t.TempDir()

	content := "[Broken](missing.md)\n"
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(index.md) error = %v", err)
	}

	report, err := ValidateDocs(docsDir)
	if err != nil {
		t.Fatalf("ValidateDocs() error = %v", err)
	}

	if len(report.BrokenLinks) != 1 {
		t.Fatalf("len(BrokenLinks) = %d, want 1", len(report.BrokenLinks))
	}
	if report.BrokenLinks[0].Line != 1 {
		t.Fatalf("Line = %d, want 1 (broken link is on the first line)", report.BrokenLinks[0].Line)
	}
}
```

- [ ] **Step 3: Run the tests to confirm RED**

Run:

```bash
go test ./internal/composer -run 'TestValidateDocsReportsCorrectLineNumbers|TestValidateDocsReportsLineOneForBrokenLinkOnFirstLine' -v
```

Expected: FAIL — both report `Line = 0` instead of the expected line numbers.

- [ ] **Step 4: Commit the RED tests**

```bash
git add internal/composer/validator_test.go
git commit -m "test: add line number expectations for broken link validation"
```

---

### Task 2: Implement line number calculation

**Files:**
- Modify: `internal/composer/validator.go:49-54`

- [ ] **Step 1: Replace `Line: 0` with computed line number**

Change the `BrokenLink` construction at validator.go:49-54 from:

```go
report.BrokenLinks = append(report.BrokenLinks, store.BrokenLink{
    SourceFile: path,
    LinkText:   linkText,
    LinkTarget: target,
    Line:       0,
})
```

to:

```go
report.BrokenLinks = append(report.BrokenLinks, store.BrokenLink{
    SourceFile: path,
    LinkText:   linkText,
    LinkTarget: target,
    Line:       bytes.Count(content[:match[0]], []byte("\n")) + 1,
})
```

This requires adding `"bytes"` to the import block in validator.go. The `content` variable is already `[]byte` (from `os.ReadFile`), so the slice `content[:match[0]]` gives all bytes before the match start. Counting newlines in that prefix gives the 0-based line index; adding 1 converts to 1-based line numbering.

- [ ] **Step 2: Run the RED tests to confirm GREEN**

Run:

```bash
go test ./internal/composer -run 'TestValidateDocsReportsCorrectLineNumbers|TestValidateDocsReportsLineOneForBrokenLinkOnFirstLine' -v
```

Expected: PASS.

- [ ] **Step 3: Run existing validator tests to confirm no regression**

Run:

```bash
go test ./internal/composer -run 'TestValidateDocs' -v
```

Expected: ALL PASS — existing tests don't assert `Line` values, so the change is transparent.

- [ ] **Step 4: Commit the fix**

```bash
git add internal/composer/validator.go
git commit -m "fix: compute line numbers for broken link validation reports"
```

---

### Task 3: Verify validator package regression

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run full composer package tests**

Run:

```bash
go test ./internal/composer -v
```

Expected: ALL PASS.

- [ ] **Step 2: Run full repository regression**

Run:

```bash
go test ./...
```

Expected: ALL PASS, zero failures.
