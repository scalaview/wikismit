package agent

import "github.com/scalaview/wikismit/internal/llm"

// ExploreConfig holds configuration for the project structure exploration agent.
type ExploreConfig struct {
	Model          string
	MaxTokens      int
	Temperature    float32
	MaxRequests    int
	Language       string
	SkeletonFilter SkeletonFilterConfig
}

// SkeletonFilterConfig controls which functions are included in the exploration skeleton.
type SkeletonFilterConfig struct {
	MinFuncLines  int     // minimum lines of code (default: 5)
	MinCalledBy   int     // minimum CalledBy count (default: 1)
	MinImportance float64 // minimum ImportanceScore (default: 0.05)
}

// ExploreRequest represents a single exploration request from the LLM.
type ExploreRequest struct {
	Type   string `json:"type"`   // "read_file" or "read_function"
	Target string `json:"target"` // file path for read_file, FuncID for read_function
	Reason string `json:"reason"` // why the LLM wants to see this
}

// ExploreResponse is the structured LLM output for the exploration phase.
type ExploreResponse struct {
	Requests []*ExploreRequest `json:"requests"`
}

// ExploreResult holds the resolved data for all requests after routing.
type ExploreResult struct {
	Requests  []ExploreRequest
	Files     map[string]*FileContent  // path -> content (for read_file)
	Functions map[string]*FuncContent   // FuncID -> content (for read_function)
}

// FileContent holds the full content of a requested file.
type FileContent struct {
	Path     string
	Language string
	Content  string
}

// FuncContent holds the full source and metadata of a requested function.
type FuncContent struct {
	FuncID    string
	Path      string
	Src       string
	Signature string
}

// ExploreAgent performs project structure exploration by sending an enriched
// skeleton to the LLM and routing its read_file/read_function requests.
type ExploreAgent struct {
	client llm.Client
	cfg    *ExploreConfig
}

// NewExploreAgent creates a new exploration agent with the given LLM client and config.
func NewExploreAgent(client llm.Client, cfg *ExploreConfig) *ExploreAgent {
	if cfg == nil {
		cfg = &ExploreConfig{}
	}
	return &ExploreAgent{client: client, cfg: cfg}
}

// Name returns the agent's name for logging and identification.
func (a *ExploreAgent) Name() string {
	return "explore"
}
