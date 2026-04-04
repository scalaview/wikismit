package store

import "time"

type FileIndex map[string]*FileEntry

type OwnerShipType int
type FunctionType int

const (
	OwnershipInternal OwnerShipType = iota
	OwnershipExternal
)

const (
	FunctionTypeRegular FunctionType = iota
	FunctionTypeMethod
)

type CallRef struct {
	Name           string        `json:"name"`
	Receiver       string        `json:"receiver,omitempty"`
	Line           int           `json:"line"`
	Ownership      OwnerShipType `json:"ownership"`
	Path           string        `json:"path,omitempty"`
	ResolvedTarget string        `json:"resolved_target,omitempty"`
}

type VarDecl struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

type FileEntry struct {
	Language    string          `json:"language"`
	ContentHash string          `json:"content_hash"`
	Functions   []*FunctionDecl `json:"functions"`
	Types       []*TypeDecl     `json:"types"`
	Imports     []*Import       `json:"imports"`
	Path        string          `json:"path"`
}

type FunctionDecl struct {
	Name         string       `json:"name"`
	Signature    string       `json:"signature"`
	LineStart    int          `json:"line_start"`
	LineEnd      int          `json:"line_end"`
	Exported     bool         `json:"exported"`
	Receiver     string       `json:"receiver,omitempty"`
	FunctionType FunctionType `json:"function_type"`
	Src          string       `json:"src"`
	Path         string       `json:"path"`
	Summary      string       `json:"summary,omitempty"`
	Calls        []*CallRef   `json:"calls,omitempty"`
	VarDefs      []*VarDecl   `json:"var_defs,omitempty"`
}

type TypeDecl struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Exported  bool   `json:"exported"`
	Src       string `json:"src"`
	Path      string `json:"path"`
}

type Import struct {
	Path         string `json:"path"`
	Internal     bool   `json:"internal"`
	ResolvedPath string `json:"-"`
	Alias        string `json:"alias,omitempty"`
}

type DepGraph map[string][]string

type Module struct {
	ID              string   `json:"id"`
	Files           []string `json:"files"`
	Shared          bool     `json:"shared"`
	Owner           string   `json:"owner"`
	DependsOnShared []string `json:"depends_on_shared,omitempty"`
	ReferencedBy    []string `json:"referenced_by,omitempty"`
}

type NavPlan struct {
	GeneratedAt time.Time `json:"generated_at"`
	Modules     []*Module `json:"modules"`
}

type SharedSummary struct {
	Summary      string         `json:"summary"`
	KeyTypes     []string       `json:"key_types"`
	KeyFunctions []*KeyFunction `json:"key_functions"`
	SourceRefs   []string       `json:"source_refs"`
}

type BrokenLink struct {
	SourceFile string `json:"source_file"`
	LinkText   string `json:"link_text"`
	LinkTarget string `json:"link_target"`
	Line       int    `json:"line"`
}

type ValidationReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	BrokenLinks []*BrokenLink `json:"broken_links"`
	TotalLinks  int           `json:"total_links"`
	TotalFiles  int           `json:"total_files"`
}

type KeyFunction struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Ref       string `json:"ref"`
}

type SharedContext map[string]*SharedSummary

type CallGraph map[string][]string
