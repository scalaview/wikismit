package analyzer

import (
	"fmt"

	"github.com/scalaview/wikismit/internal/analyzer/lang"
	"github.com/scalaview/wikismit/pkg/store"
)

type LanguageParser interface {
	Extensions() []string
	ExtractSymbols(path string, src []byte) (store.FileEntry, error)
}

var registry = map[string]LanguageParser{}

func init() {
	lang.SetGoParserRegister(func(parser interface {
		Extensions() []string
		ExtractSymbols(path string, src []byte) (store.FileEntry, error)
	}) {
		Register(parser)
	})
}

func Register(p LanguageParser) {
	for _, extension := range p.Extensions() {
		if _, exists := registry[extension]; exists {
			panic(fmt.Sprintf("parser already registered for extension %q", extension))
		}
		registry[extension] = p
	}
}

func Lookup(extension string) (LanguageParser, bool) {
	p, ok := registry[extension]
	return p, ok
}
