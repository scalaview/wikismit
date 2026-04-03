package lang

import (
	"strings"

	"github.com/scalaview/wikismit/pkg/store"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extractor interface {
	Execute(captureMap map[string]*sitter.Node, src []byte, entry *store.FileEntry, relPath string, srcSplitter *srcSplitter) bool
}

type FunctionExtractor struct {
	decl string
	name string
}

func (e *FunctionExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *store.FileEntry, relPath string, srcSplitter *srcSplitter) bool {
	if functionNode, ok := captureMap[e.decl]; ok {
		nameNode := captureMap[e.name]
		name := nameNode.Utf8Text(src)
		entry.Functions = append(entry.Functions, &store.FunctionDecl{
			Name:         name,
			Signature:    sourceForNode(src, functionNode),
			LineStart:    lineNumber(functionNode.StartPosition()),
			LineEnd:      lineNumber(functionNode.EndPosition()),
			Exported:     isExported(name),
			FunctionType: store.FunctionTypeRegular,
			Path:         relPath,
			Src:          srcSplitter.extractInnerBodies(lineNumber(functionNode.StartPosition()), lineNumber(functionNode.EndPosition())),
		})
		return true
	}

	return false
}

type TypeExtractor struct {
	decl string
	name string
	kind string
}

func (e *TypeExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *store.FileEntry, relPath string, srcSplitter *srcSplitter) bool {
	if typeNode, ok := captureMap[e.decl]; ok {
		nameNode := captureMap[e.name]
		kindNode := captureMap[e.kind]
		name := nameNode.Utf8Text(src)
		entry.Types = append(entry.Types, &store.TypeDecl{
			Name:      name,
			Kind:      typeKind(kindNode),
			LineStart: lineNumber(typeNode.StartPosition()),
			LineEnd:   lineNumber(typeNode.EndPosition()),
			Exported:  isExported(name),
			Path:      relPath,
			Src:       srcSplitter.extractInnerBodies(lineNumber(typeNode.StartPosition()), lineNumber(typeNode.EndPosition())),
		})
		return true
	}

	return false
}

type ImportExtractor struct {
	path string
}

func (e *ImportExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *store.FileEntry, relPath string, srcSplitter *srcSplitter) bool {
	if importNode, ok := captureMap[e.path]; ok {
		entry.Imports = append(entry.Imports, &store.Import{
			Path:     strings.Trim(importNode.Utf8Text(src), `"`),
			Internal: false,
		})
		return true
	}
	return false
}

type AliasExtractor struct {
	decl string
	name string
}

func (e *AliasExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *store.FileEntry, relPath string, srcSplitter *srcSplitter) bool {
	if aliasNode, ok := captureMap[e.decl]; ok {
		nameNode := captureMap[e.name]
		name := nameNode.Utf8Text(src)
		entry.Types = append(entry.Types, &store.TypeDecl{
			Name:      name,
			Kind:      "alias",
			LineStart: lineNumber(aliasNode.StartPosition()),
			LineEnd:   lineNumber(aliasNode.EndPosition()),
			Exported:  isExported(name),
			Path:      relPath,
			Src:       srcSplitter.extractInnerBodies(lineNumber(aliasNode.StartPosition()), lineNumber(aliasNode.EndPosition())),
		})
		return true
	}
	return false
}

type MethodExtractor struct {
	decl     string
	name     string
	receiver string
}

func (e *MethodExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *store.FileEntry, relPath string, srcSplitter *srcSplitter) bool {
	if methodNode, ok := captureMap[e.decl]; ok {
		nameNode := captureMap[e.name]
		name := nameNode.Utf8Text(src)
		receiver := ""
		if recvNode, ok := captureMap[e.receiver]; ok {
			receiver = recvNode.Utf8Text(src)
		}
		entry.Functions = append(entry.Functions, &store.FunctionDecl{
			Name:         name,
			Signature:    sourceForNode(src, methodNode),
			LineStart:    lineNumber(methodNode.StartPosition()),
			LineEnd:      lineNumber(methodNode.EndPosition()),
			Exported:     isExported(name),
			Receiver:     receiver,
			FunctionType: store.FunctionTypeMethod,
			Path:         relPath,
			Src:          srcSplitter.extractInnerBodies(lineNumber(methodNode.StartPosition()), lineNumber(methodNode.EndPosition())),
		})
		return true
	}

	return false
}

func NewExtractors() []Extractor {
	return []Extractor{
		&FunctionExtractor{
			decl: "function.decl",
			name: "function.name",
		},
		&MethodExtractor{
			decl:     "method.decl",
			name:     "method.name",
			receiver: "method.receiver",
		},
		&TypeExtractor{
			decl: "type.decl",
			name: "type.name",
			kind: "type.kind",
		},
		&AliasExtractor{
			decl: "alias.decl",
			name: "alias.name",
		},
		&ImportExtractor{
			path: "import.path",
		},
	}
}
