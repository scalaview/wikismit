package analyzer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/scalaview/wikismit/internal/analyzer/lang"
	"github.com/scalaview/wikismit/pkg/store"
)

type testParser struct {
	extensions []string
}

func (p *testParser) Extensions() []string {
	return p.extensions
}

func (p *testParser) ExtractSymbols(path string, src []byte) (store.FileEntry, error) {
	return store.FileEntry{}, nil
}

func TestRegisterIndexesAllExtensions(t *testing.T) {
	registry = map[string]LanguageParser{}
	t.Cleanup(func() {
		registry = map[string]LanguageParser{}
	})

	parser := &testParser{extensions: []string{".go", ".mod"}}
	Register(parser)

	for _, extension := range parser.Extensions() {
		got, ok := registry[extension]
		if !ok {
			t.Fatalf("registry missing extension %q", extension)
		}
		if got != parser {
			t.Fatalf("registry[%q] = %p, want %p", extension, got, parser)
		}
	}
}

func TestRegisterRejectsDuplicateExtension(t *testing.T) {
	registry = map[string]LanguageParser{}
	t.Cleanup(func() {
		registry = map[string]LanguageParser{}
	})

	Register(&testParser{extensions: []string{".go"}})

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("Register() did not panic on duplicate extension")
		}
	}()

	Register(&testParser{extensions: []string{".go"}})
}

func TestTypeDeclCarriesLineEnd(t *testing.T) {
	entry := store.FileEntry{
		Types: []store.TypeDecl{{
			Name:      "Widget",
			Kind:      "struct",
			LineStart: 3,
			LineEnd:   8,
			Exported:  true,
		}},
		Imports: []store.Import{{
			Path:         "github.com/scalaview/wikismit/pkg/store",
			Internal:     true,
			ResolvedPath: "pkg/store/artifacts.go",
		}},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonText := string(data)
	if !strings.Contains(jsonText, `"line_end":8`) {
		t.Fatalf("marshaled json = %s, want type line_end field", jsonText)
	}
	if strings.Contains(jsonText, "ResolvedPath") || strings.Contains(jsonText, "resolved_path") {
		t.Fatalf("marshaled json = %s, resolved path should not be serialized", jsonText)
	}
}

func TestGoParserRegistersGoExtension(t *testing.T) {
	registry = map[string]LanguageParser{}
	t.Cleanup(func() {
		registry = map[string]LanguageParser{}
	})

	lang.SetGoParserRegister(func(parser interface {
		Extensions() []string
		ExtractSymbols(path string, src []byte) (store.FileEntry, error)
	}) {
		Register(parser)
	})

	got, ok := Lookup(".go")
	if !ok {
		t.Fatal("Lookup(.go) = missing, want registered parser")
	}
	if got == nil {
		t.Fatal("Lookup(.go) = nil, want parser")
	}
}
