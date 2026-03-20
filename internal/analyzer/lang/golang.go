package lang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/scalaview/wikismit/pkg/store"
	sitter "github.com/tree-sitter/go-tree-sitter"
	treeSitterGo "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

const simpleGoQuery = `
(function_declaration
  name: (identifier) @function.name) @function.decl

(type_spec
  name: (type_identifier) @type.name
  type: (struct_type) @type.kind) @type.decl

(import_spec
  path: (interpreted_string_literal) @import.path) @import.decl
`

type goParser struct{}

var registerGoParser func(interface {
	Extensions() []string
	ExtractSymbols(path string, src []byte) (store.FileEntry, error)
})

func SetGoParserRegister(register func(interface {
	Extensions() []string
	ExtractSymbols(path string, src []byte) (store.FileEntry, error)
})) {
	registerGoParser = register
	if registerGoParser != nil {
		registerGoParser(&goParser{})
	}
}

func newGoParser() *sitter.Parser {
	parser := sitter.NewParser()
	language := sitter.NewLanguage(treeSitterGo.Language())
	if err := parser.SetLanguage(language); err != nil {
		parser.Close()
		panic(fmt.Sprintf("set Go parser language: %v", err))
	}
	return parser
}

func (p *goParser) Extensions() []string {
	return []string{".go"}
}

func (p *goParser) ExtractSymbols(path string, src []byte) (store.FileEntry, error) {
	parser := newGoParser()
	defer parser.Close()

	tree := parser.Parse(src, nil)
	defer tree.Close()

	query, queryErr := sitter.NewQuery(sitter.NewLanguage(treeSitterGo.Language()), simpleGoQuery)
	if queryErr != nil {
		return store.FileEntry{}, fmt.Errorf("build Go query: %w", queryErr)
	}
	defer query.Close()

	queryCursor := sitter.NewQueryCursor()
	defer queryCursor.Close()

	functions := []store.FunctionDecl{}
	types := []store.TypeDecl{}
	imports := []store.Import{}

	matches := queryCursor.Matches(query, tree.RootNode(), src)
	for match := matches.Next(); match != nil; match = matches.Next() {
		captureMap := capturesByName(query, match)

		if functionNode, ok := captureMap["function.decl"]; ok {
			nameNode := captureMap["function.name"]
			name := nameNode.Utf8Text(src)
			functions = append(functions, store.FunctionDecl{
				Name:      name,
				Signature: sourceForNode(src, functionNode),
				LineStart: lineNumber(functionNode.StartPosition()),
				LineEnd:   lineNumber(functionNode.EndPosition()),
				Exported:  isExported(name),
			})
			continue
		}

		if typeNode, ok := captureMap["type.decl"]; ok {
			nameNode := captureMap["type.name"]
			name := nameNode.Utf8Text(src)
			types = append(types, store.TypeDecl{
				Name:      name,
				Kind:      "struct",
				LineStart: lineNumber(typeNode.StartPosition()),
				LineEnd:   lineNumber(typeNode.EndPosition()),
				Exported:  isExported(name),
			})
			continue
		}

		if importNode, ok := captureMap["import.path"]; ok {
			imports = append(imports, store.Import{
				Path:     strings.Trim(importNode.Utf8Text(src), `"`),
				Internal: false,
			})
		}
	}

	return store.FileEntry{
		Language:    "go",
		ContentHash: contentHash(src),
		Functions:   functions,
		Types:       types,
		Imports:     imports,
	}, nil
}

func contentHash(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}

func capturesByName(query *sitter.Query, match *sitter.QueryMatch) map[string]*sitter.Node {
	result := map[string]*sitter.Node{}
	names := query.CaptureNames()
	for _, capture := range match.Captures {
		captureNode := capture.Node
		result[names[capture.Index]] = &captureNode
	}
	return result
}

func sourceForNode(src []byte, node *sitter.Node) string {
	startByte, endByte := node.ByteRange()
	text := string(src[startByte:endByte])
	if bodyIndex := strings.Index(text, " {"); bodyIndex >= 0 {
		return text[:bodyIndex]
	}
	return strings.TrimSpace(text)
}

func lineNumber(point sitter.Point) int {
	return int(point.Row) + 1
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper([]rune(name)[0])
}
