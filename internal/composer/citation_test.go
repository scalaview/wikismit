package composer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func TestBuildSymbolMapIncludesFunctionAndTypeRefs(t *testing.T) {
	// Create a sample FileIndex with both function and type declarations
	idx := store.FileIndex{
		"internal/auth/jwt.go": store.FileEntry{
			Functions: []store.FunctionDecl{
				{Name: "GenerateToken", LineStart: 24, Exported: true},
				{Name: "validateToken", LineStart: 50, Exported: false},
			},
			Types: []store.TypeDecl{
				{Name: "Claims", LineStart: 10, Exported: true},
				{Name: "privateClaim", LineStart: 15, Exported: false},
			},
		},
		"internal/db/conn.go": store.FileEntry{
			Functions: []store.FunctionDecl{
				{Name: "Connect", LineStart: 5, Exported: true},
			},
			Types: []store.TypeDecl{
				{Name: "Config", LineStart: 1, Exported: true},
			},
		},
	}

	symbolMap := buildSymbolMap(idx)

	// Verify exported symbols are included
	expectedCount := 4 // GenerateToken, Claims, Connect, Config
	if len(symbolMap) != expectedCount {
		t.Errorf("Expected %d symbols, got %d", expectedCount, len(symbolMap))
	}

	// Verify correct format: path#Lline
	if symbolMap["GenerateToken"] != "internal/auth/jwt.go#L24" {
		t.Errorf("GenerateToken: expected 'internal/auth/jwt.go#L24', got '%s'", symbolMap["GenerateToken"])
	}
	if symbolMap["Claims"] != "internal/auth/jwt.go#L10" {
		t.Errorf("Claims: expected 'internal/auth/jwt.go#L10', got '%s'", symbolMap["Claims"])
	}
	if symbolMap["Connect"] != "internal/db/conn.go#L5" {
		t.Errorf("Connect: expected 'internal/db/conn.go#L5', got '%s'", symbolMap["Connect"])
	}
	if symbolMap["Config"] != "internal/db/conn.go#L1" {
		t.Errorf("Config: expected 'internal/db/conn.go#L1', got '%s'", symbolMap["Config"])
	}

	// Verify unexported symbols are NOT included
	if _, exists := symbolMap["validateToken"]; exists {
		t.Error("validateToken (unexported) should not be in symbol map")
	}
	if _, exists := symbolMap["privateClaim"]; exists {
		t.Error("privateClaim (unexported) should not be in symbol map")
	}
}

func TestInjectCitationsReplacesExportedBacktickSymbols(t *testing.T) {
	symbolMap := map[string]string{
		"GenerateToken": "internal/auth/jwt.go#L24",
		"Claims":        "internal/auth/jwt.go#L10",
		"Connect":       "internal/db/conn.go#L5",
	}

	content := "Use `GenerateToken` to create JWT tokens with `Claims`. Also call `Connect`."
	expected := "Use [GenerateToken](internal/auth/jwt.go#L24) to create JWT tokens with [Claims](internal/auth/jwt.go#L10). Also call [Connect](internal/db/conn.go#L5)."

	result := InjectCitations(content, symbolMap)

	if result != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, result)
	}
}

func TestInjectCitationsSkipsAlreadyLinkedAndUnknownNames(t *testing.T) {
	symbolMap := map[string]string{
		"GenerateToken": "internal/auth/jwt.go#L24",
	}

	content := "Keep [GenerateToken](internal/auth/jwt.go#L24) and leave `UnknownFunc` alone."
	result := InjectCitations(content, symbolMap)

	if result != content {
		t.Errorf("expected unchanged content\nwant: %s\n got: %s", content, result)
	}
}

func TestInjectCitationsReplacesMultipleOccurrences(t *testing.T) {
	symbolMap := map[string]string{
		"GenerateToken": "internal/auth/jwt.go#L24",
	}

	content := "`GenerateToken` appears twice: `GenerateToken`."
	result := InjectCitations(content, symbolMap)

	if count := strings.Count(result, "[GenerateToken](internal/auth/jwt.go#L24)"); count != 2 {
		t.Fatalf("expected 2 replacements, got %d in %q", count, result)
	}
}

func TestInjectCitationsSkipsLowercaseIdentifiers(t *testing.T) {
	symbolMap := map[string]string{
		"myHelper": "internal/auth/jwt.go#L50",
	}

	content := "leave `myHelper` alone"
	result := InjectCitations(content, symbolMap)

	if result != content {
		t.Errorf("expected lowercase identifier to remain unchanged\nwant: %s\n got: %s", content, result)
	}
}

func TestBuildSymbolMapPrefersExportedSymbolForAmbiguousName(t *testing.T) {
	idx := store.FileIndex{
		"pkg/beta/exported.go": {
			Functions: []store.FunctionDecl{{Name: "Normalize", LineStart: 20, Exported: true}},
		},
		"pkg/alpha/unexported.go": {
			Functions: []store.FunctionDecl{{Name: "Normalize", LineStart: 10, Exported: false}},
		},
		"pkg/alpha/exported.go": {
			Functions: []store.FunctionDecl{{Name: "Normalize", LineStart: 5, Exported: true}},
		},
	}

	symbolMap := buildSymbolMap(idx)

	if got := symbolMap["Normalize"]; got != "pkg/alpha/exported.go#L5" {
		t.Fatalf("expected canonical exported alphabetical ref, got %q", got)
	}
}

func TestProcessFileOverwritesMarkdownWithInjectedCitations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.md")
	original := "Use `GenerateToken` to issue tokens."
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	symbolMap := map[string]string{
		"GenerateToken": "internal/auth/jwt.go#L24",
	}

	if err := ProcessFile(path, symbolMap); err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read processed file: %v", err)
	}

	expected := "Use [GenerateToken](internal/auth/jwt.go#L24) to issue tokens."
	if got := string(data); got != expected {
		t.Fatalf("processed file = %q, want %q", got, expected)
	}
}
