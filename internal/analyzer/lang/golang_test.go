package lang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestExtractSymbolsMatchesSimpleGolden(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "testdata", "fixtures", "golang", "simple.go")
	goldenPath := filepath.Join("..", "..", "..", "testdata", "fixtures", "golang", "simple.golden.json")

	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", fixturePath, err)
	}
	parser := &goParser{}
	got, err := parser.ExtractSymbols("simple.go", src)
	if err != nil {
		t.Fatalf("ExtractSymbols() error = %v", err)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", goldenPath, err)
	}

	gotJSON, err := marshalGolden(got)
	if err != nil {
		t.Fatalf("marshalGolden() error = %v", err)
	}

	if diff := cmp.Diff(string(want), string(gotJSON)); diff != "" {
		t.Fatalf("simple golden mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractSymbolsMatchesComplexGolden(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "testdata", "fixtures", "golang", "complex.go")
	goldenPath := filepath.Join("..", "..", "..", "testdata", "fixtures", "golang", "complex.golden.json")

	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", fixturePath, err)
	}
	parser := &goParser{}
	got, err := parser.ExtractSymbols("complex.go", src)
	if err != nil {
		t.Fatalf("ExtractSymbols() error = %v", err)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", goldenPath, err)
	}

	gotJSON, err := marshalGolden(got)
	if err != nil {
		t.Fatalf("marshalGolden() error = %v", err)
	}

	if diff := cmp.Diff(string(want), string(gotJSON)); diff != "" {
		t.Fatalf("complex golden mismatch (-want +got):\n%s", diff)
	}
}

func marshalGolden(entry store.FileEntry) ([]byte, error) {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
