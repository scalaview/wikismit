package lang

import "testing"

func TestLinesOfCodeExtractorCountsNonEmptyLines(t *testing.T) {
	ext := &LinesOfCodeExtractor{}
	src := "line 1\n\nline 3\nline 4\n"
	got := ext.LinesOfCode(src)
	if got != 3 {
		t.Fatalf("LinesOfCode() = %d, want 3 (3 non-empty lines out of 4)", got)
	}
}

func TestLinesOfCodeExtractorReturnsZeroForEmpty(t *testing.T) {
	ext := &LinesOfCodeExtractor{}
	if got := ext.LinesOfCode(""); got != 0 {
		t.Fatalf("LinesOfCode(\"\") = %d, want 0", got)
	}
}

func TestLinesOfCodeExtractorCountsSingleLine(t *testing.T) {
	ext := &LinesOfCodeExtractor{}
	if got := ext.LinesOfCode("func foo() {}"); got != 1 {
		t.Fatalf("LinesOfCode(single line) = %d, want 1", got)
	}
}
