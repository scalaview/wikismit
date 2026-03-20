package lang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/scalaview/wikismit/pkg/store"
	sitter "github.com/tree-sitter/go-tree-sitter"
	treeSitterGo "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

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

	return store.FileEntry{
		Language:    "go",
		ContentHash: contentHash(src),
		Functions:   []store.FunctionDecl{},
		Types:       []store.TypeDecl{},
		Imports:     []store.Import{},
	}, nil
}

func contentHash(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}
