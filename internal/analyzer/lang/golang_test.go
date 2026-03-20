package lang

import (
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func TestNewGoParserParsesMinimalSource(t *testing.T) {
	parser := newGoParser()
	defer parser.Close()

	tree := parser.Parse([]byte("package main\n"), nil)
	defer tree.Close()

	if tree.RootNode().Kind() != "source_file" {
		t.Fatalf("RootNode().Kind() = %q, want 'source_file'", tree.RootNode().Kind())
	}
	if parser.Language() == nil {
		t.Fatal("Language() = nil, want configured language")
	}
}

func TestContentHashIsStableForSameBytes(t *testing.T) {
	first := contentHash([]byte("package main\n"))
	second := contentHash([]byte("package main\n"))
	third := contentHash([]byte("package widgets\n"))

	if first != second {
		t.Fatalf("contentHash() = %q, second = %q, want equal hashes for same bytes", first, second)
	}
	if first == third {
		t.Fatalf("contentHash() = %q, third = %q, want different hashes for different bytes", first, third)
	}
}

func TestExtractSymbolsReturnsLanguageAndHashForEmptyGoFile(t *testing.T) {
	parser := &goParser{}
	src := []byte("package main\n")

	entry, err := parser.ExtractSymbols("main.go", src)
	if err != nil {
		t.Fatalf("ExtractSymbols() error = %v", err)
	}

	if entry.Language != "go" {
		t.Fatalf("Language = %q, want %q", entry.Language, "go")
	}
	if entry.ContentHash != contentHash(src) {
		t.Fatalf("ContentHash = %q, want %q", entry.ContentHash, contentHash(src))
	}
	if entry.Functions == nil {
		t.Fatal("Functions = nil, want empty slice")
	}
	if entry.Types == nil {
		t.Fatal("Types = nil, want empty slice")
	}
	if entry.Imports == nil {
		t.Fatal("Imports = nil, want empty slice")
	}
	if len(entry.Functions) != 0 || len(entry.Types) != 0 || len(entry.Imports) != 0 {
		t.Fatalf("ExtractSymbols() = %+v, want empty symbol slices", entry)
	}

	var _ store.FileEntry = entry
}
