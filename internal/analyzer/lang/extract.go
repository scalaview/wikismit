package lang

import (
	"strings"

	"github.com/scalaview/wikismit/pkg/store"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

type ExtractResult struct {
	Functions []*store.FunctionDecl `json:"functions"`
	Types     []*store.TypeDecl     `json:"types"`
	Imports   []*store.Import       `json:"imports"`
	Calls     []*store.CallRef      `json:"calls"`
	VarDecls  []*store.VarDecl      `json:"var_decls"`
}

type Extractor interface {
	Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool
}

type FunctionExtractor struct {
	decl string
	name string
}

func (e *FunctionExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
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

func (e *TypeExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
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

func (e *ImportExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
	if importNode, ok := captureMap[e.path]; ok {
		entry.Imports = append(entry.Imports, &store.Import{
			Path:     strings.Trim(importNode.Utf8Text(src), `"`),
			Internal: false,
		})
		return true
	}
	return false
}

type ImportAliasExtractor struct {
	decl  string
	alias string
	path  string
}

func (e *ImportAliasExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
	if _, ok := captureMap[e.decl]; ok {
		aliasText := ""
		if aliasNode, ok := captureMap[e.alias]; ok {
			aliasText = aliasNode.Utf8Text(src)
		}
		pathNode := captureMap[e.path]
		entry.Imports = append(entry.Imports, &store.Import{
			Path:     strings.Trim(pathNode.Utf8Text(src), `"`),
			Internal: false,
			Alias:    aliasText,
		})
		return true
	}
	return false
}

type AliasExtractor struct {
	decl string
	name string
}

func (e *AliasExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
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
	decl         string
	name         string
	receiver     string
	receiverName string
}

func (e *MethodExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
	if methodNode, ok := captureMap[e.decl]; ok {
		nameNode := captureMap[e.name]
		name := nameNode.Utf8Text(src)
		receiver := ""
		if recvNode, ok := captureMap[e.receiver]; ok {
			receiver = recvNode.Utf8Text(src)
		}
		recvName := ""
		if recvNameNode, ok := captureMap[e.receiverName]; ok {
			recvName = recvNameNode.Utf8Text(src)
		}
		fn := &store.FunctionDecl{
			Name:         name,
			Signature:    sourceForNode(src, methodNode),
			LineStart:    lineNumber(methodNode.StartPosition()),
			LineEnd:      lineNumber(methodNode.EndPosition()),
			Exported:     isExported(name),
			Receiver:     receiver,
			FunctionType: store.FunctionTypeMethod,
			Path:         relPath,
			Src:          srcSplitter.extractInnerBodies(lineNumber(methodNode.StartPosition()), lineNumber(methodNode.EndPosition())),
		}
		entry.Functions = append(entry.Functions, fn)

		// Register receiver parameter as implicit VarDecl
		if recvName != "" && receiver != "" {
			fn.VarDefs = append(fn.VarDefs, &store.VarDecl{
				Name: recvName,
				Type: receiver,
				Line: fn.LineStart,
			})
		}

		return true
	}

	return false
}

type CallExtractor struct {
	decl string
	name string
	recv string
}

func (e *CallExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
	if callNode, ok := captureMap[e.decl]; ok {
		receiver := ""
		methodNode := captureMap[e.name]
		if e.recv != "" {
			recvNode := captureMap[e.recv]
			receiver = recvNode.Utf8Text(src)
		}
		name := methodNode.Utf8Text(src)

		entry.Calls = append(entry.Calls, &store.CallRef{
			Name:      name,
			Receiver:  receiver,
			Line:      lineNumber(callNode.StartPosition()),
			Ownership: store.OwnershipExternal,
		})
		return true
	}
	return false
}

type VarExtractor struct {
	decl string
	name string
	typ  string
	pkg  string
}

func (e *VarExtractor) Execute(captureMap map[string]*sitter.Node, src []byte, entry *ExtractResult, relPath string, srcSplitter *srcSplitter) bool {
	if _, ok := captureMap[e.decl]; ok {
		nameNode := captureMap[e.name]
		typeNode := captureMap[e.typ]
		typeName := typeNode.Utf8Text(src)
		if e.pkg != "" {
			pkgNode := captureMap[e.pkg]
			typeName = pkgNode.Utf8Text(src) + "." + typeNode.Utf8Text(src)
		}

		entry.VarDecls = append(entry.VarDecls, &store.VarDecl{
			Name: nameNode.Utf8Text(src),
			Type: typeName,
			Line: lineNumber(nameNode.StartPosition()),
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
			decl:         "method.decl",
			name:         "method.name",
			receiver:     "method.receiver",
			receiverName: "method.receiver.name",
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
		&ImportAliasExtractor{
			decl:  "import.alias.decl",
			alias: "import.alias",
			path:  "import.alias.path",
		},
		&CallExtractor{
			decl: "call.expr",
			name: "call.name",
		},
		&CallExtractor{
			decl: "call.selector.expr",
			name: "call.method",
			recv: "call.receiver",
		},
		&VarExtractor{
			decl: "var.decl",
			name: "var.name",
			typ:  "var.type",
		},
		&VarExtractor{
			decl: "var.qualified.decl",
			name: "var.name",
			typ:  "var.type",
			pkg:  "var.pkg",
		},
		&VarExtractor{
			decl: "var.ptr.decl",
			name: "var.name",
			typ:  "var.type",
		},
		&VarExtractor{
			decl: "var.ptr.qualified.decl",
			name: "var.name",
			typ:  "var.type",
			pkg:  "var.pkg",
		},
		&VarExtractor{
			decl: "var.composite.decl",
			name: "var.name",
			typ:  "var.type",
		},
		&VarExtractor{
			decl: "var.qualified.composite.decl",
			name: "var.name",
			typ:  "var.type",
			pkg:  "var.pkg",
		},
	}
}
