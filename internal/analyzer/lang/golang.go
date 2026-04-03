package lang

import (
	"bytes"
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

(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      name: (identifier) @method.receiver.name
      type: (type_identifier) @method.receiver))
  name: (field_identifier) @method.name) @method.decl

(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      name: (identifier) @method.receiver.name
      type: (pointer_type (type_identifier) @method.receiver)))
  name: (field_identifier) @method.name) @method.decl

(type_spec
  name: (type_identifier) @type.name
  type: (_) @type.kind) @type.decl

(type_alias
  name: (type_identifier) @alias.name
  type: (_) @alias.kind) @alias.decl

(import_spec
  path: (interpreted_string_literal) @import.path) @import.decl

(import_spec
  name: [
    (package_identifier)
    (blank_identifier)
    (dot)
  ] @import.alias
  path: (interpreted_string_literal) @import.alias.path) @import.alias.decl

(call_expression
  function: (identifier) @call.name) @call.expr

(call_expression
  function: (selector_expression
    operand: (identifier) @call.receiver
    field: (field_identifier) @call.method)) @call.selector.expr

(var_spec
  name: (identifier) @var.name
  type: (type_identifier) @var.type) @var.decl

(var_spec
  name: (identifier) @var.name
  type: (qualified_type
    package: (package_identifier) @var.pkg
    name: (type_identifier) @var.type)) @var.qualified.decl

(var_spec
  name: (identifier) @var.name
  type: (pointer_type (type_identifier) @var.type)) @var.ptr.decl

(var_spec
  name: (identifier) @var.name
  type: (pointer_type (qualified_type
    package: (package_identifier) @var.pkg
    name: (type_identifier) @var.type))) @var.ptr.qualified.decl

(short_var_declaration
  left: (expression_list
    (identifier) @var.name)
  right: (expression_list
    (composite_literal
      type: (type_identifier) @var.type))) @var.composite.decl

(short_var_declaration
  left: (expression_list
    (identifier) @var.name)
  right: (expression_list
    (composite_literal
      type: (qualified_type
        package: (package_identifier) @var.pkg
        name: (type_identifier) @var.type)))) @var.qualified.composite.decl
`

type goParser struct {
	extractors []Extractor
}

var registerGoParser func(interface {
	Extensions() []string
	ExtractSymbols(path string, relPath string, src []byte) (*store.FileEntry, error)
})

func SetGoParserRegister(register func(interface {
	Extensions() []string
	ExtractSymbols(path string, relPath string, src []byte) (*store.FileEntry, error)
})) {
	registerGoParser = register
	if registerGoParser != nil {
		registerGoParser(newGoParser())
	}
}

func newGoParser() *goParser {
	return &goParser{
		extractors: NewExtractors(),
	}
}

func newTreeSitterParser() *sitter.Parser {
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

func (p *goParser) ExtractSymbols(path string, relPath string, src []byte) (*store.FileEntry, error) {
	parser := newTreeSitterParser()
	defer parser.Close()
	srcSplitter := newSrcSplitter(src)

	tree := parser.Parse(src, nil)
	defer tree.Close()
	if tree.RootNode().HasError() {
		return nil, fmt.Errorf("parse Go file %q: syntax error", path)
	}

	query, queryErr := sitter.NewQuery(sitter.NewLanguage(treeSitterGo.Language()), simpleGoQuery)
	if queryErr != nil {
		return nil, fmt.Errorf("build Go query: %w", queryErr)
	}
	defer query.Close()

	queryCursor := sitter.NewQueryCursor()
	defer queryCursor.Close()

	result := &ExtractResult{
		Functions: make([]*store.FunctionDecl, 0),
		Types:     make([]*store.TypeDecl, 0),
		Imports:   make([]*store.Import, 0),
		Calls:     make([]*store.CallRef, 0),
		VarDecls:  make([]*store.VarDecl, 0),
	}

	matches := queryCursor.Matches(query, tree.RootNode(), src)
	for match := matches.Next(); match != nil; match = matches.Next() {
		captureMap := capturesByName(query, match)

		// Try registered extractors first
		for _, extractor := range p.extractors {
			if extractor.Execute(captureMap, src, result, relPath, srcSplitter) {
				continue
			}
		}
	}

	entry := &store.FileEntry{
		Language:    "go",
		ContentHash: contentHash(src),
		Functions:   result.Functions,
		Types:       result.Types,
		Imports:     mergeImports(result.Imports),
		Path:        path,
	}

	// Scope calls and var decls to their enclosing functions
	scopeToFunctions(entry, result.Calls, result.VarDecls)

	return entry, nil
}

func mergeImports(imports []*store.Import) []*store.Import {
	merged := make([]*store.Import, 0, len(imports))
	byPath := make(map[string]int, len(imports))
	for _, imp := range imports {
		if idx, ok := byPath[imp.Path]; ok {
			if imp.Alias != "" {
				merged[idx].Alias = imp.Alias
			}
			continue
		}
		byPath[imp.Path] = len(merged)
		merged = append(merged, imp)
	}
	return merged
}

// scopeToFunctions assigns each CallRef and VarDecl to the FunctionDecl that encloses it by line range.
func scopeToFunctions(entry *store.FileEntry, calls []*store.CallRef, varDecls []*store.VarDecl) {
	for _, fn := range entry.Functions {
		// Scope calls
		for _, call := range calls {
			if call.Line >= fn.LineStart && call.Line <= fn.LineEnd {
				fn.Calls = append(fn.Calls, call)
			}
		}
		// Scope var decls (skip the implicit receiver var at fn.LineStart)
		for _, vd := range varDecls {
			if vd.Line >= fn.LineStart && vd.Line <= fn.LineEnd {
				fn.VarDefs = append(fn.VarDefs, vd)
			}
		}
	}
}

type srcSplitter struct {
	lines [][]byte
}

func newSrcSplitter(src []byte) *srcSplitter {
	return &srcSplitter{
		lines: bytes.Split(src, []byte("\n")),
	}
}

func (s *srcSplitter) extractInnerBodies(start int, end int) string {
	lines := s.lines
	var b strings.Builder
	start = start - 1
	if end > len(lines) {

		end = len(lines)
	}
	for _, line := range lines[start:end] {
		b.Write(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
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

func typeKind(node *sitter.Node) string {
	if node == nil {
		return "alias"
	}
	switch node.Kind() {
	case "struct_type":
		return "struct"
	case "interface_type":
		return "interface"
	default:
		return "alias"
	}
}
