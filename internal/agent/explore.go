package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	promptpkg "github.com/scalaview/wikismit/internal/agent/prompt"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

// SkeletonBuilderFunc builds the exploration skeleton. Injected by the planner's
// agent factory to break the circular import (planner → agent → planner).
var SkeletonBuilderFunc = func(idx store.FileIndex, maxTokens int, filter *metrics.ImportanceFilter, cfg SkeletonFilterConfig) string {
	return "" // default: no-op, factory overrides
}

// Run executes the project structure exploration.
func (a *ExploreAgent) Run(ctx context.Context, idx store.FileIndex, graph store.CallGraph, metricsData store.MetricsMap) (*ExploreResult, error) {
	var filter *metrics.ImportanceFilter
	if len(metricsData) > 0 {
		filter = metrics.NewImportanceFilter(metricsData, 0)
	}

	skeleton := SkeletonBuilderFunc(idx, 3000, filter, a.cfg.SkeletonFilter)

	req, err := a.buildPrompt(skeleton)
	if err != nil {
		return nil, fmt.Errorf("explore build prompt: %w", err)
	}

	response, err := a.client.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("explore LLM call: %w", err)
	}

	resp, err := a.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("explore parse response: %w", err)
	}

	maxReqs := a.cfg.MaxRequests
	if maxReqs <= 0 {
		maxReqs = 5
	}
	if len(resp.Requests) > maxReqs {
		resp.Requests = resp.Requests[:maxReqs]
	}

	return a.routeRequests(idx, resp.Requests)
}

// buildPrompt renders the system and user prompt templates.
func (a *ExploreAgent) buildPrompt(skeleton string) (*llm.CompletionRequest, error) {
	var sysBuf bytes.Buffer
	if err := promptpkg.ExploreSystemPromptTmp.Execute(&sysBuf, &promptpkg.ExploreSystemPromptData{
		Language: a.cfg.Language,
	}); err != nil {
		return nil, fmt.Errorf("execute explore system prompt: %w", err)
	}

	var userBuf bytes.Buffer
	if err := promptpkg.ExploreUserPromptTmp.Execute(&userBuf, &promptpkg.ExploreUserPromptData{
		Skeleton: skeleton,
	}); err != nil {
		return nil, fmt.Errorf("execute explore user prompt: %w", err)
	}

	return &llm.CompletionRequest{
		Model:       a.cfg.Model,
		SystemMsg:   sysBuf.String(),
		UserMsg:     userBuf.String(),
		MaxTokens:   a.cfg.MaxTokens,
		Temperature: a.cfg.Temperature,
	}, nil
}

// parseResponse extracts structured exploration requests from LLM output.
func (a *ExploreAgent) parseResponse(content string) (*ExploreResponse, error) {
	var resp ExploreResponse
	if err := llm.ParseJSON(content, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// routeRequests resolves each LLM request against the FileIndex.
func (a *ExploreAgent) routeRequests(idx store.FileIndex, requests []*ExploreRequest) (*ExploreResult, error) {
	result := &ExploreResult{
		Files:     make(map[string]*FileContent),
		Functions: make(map[string]*FuncContent),
	}

	for _, req := range requests {
		if req == nil {
			continue
		}
		result.Requests = append(result.Requests, *req)

		switch req.Type {
		case "read_file":
			a.resolveReadFile(idx, req, result)
		case "read_function":
			a.resolveReadFunction(idx, req, result)
		default:
			return nil, fmt.Errorf("unsupported request type %q", req.Type)
		}
	}

	return result, nil
}

func (a *ExploreAgent) resolveReadFile(idx store.FileIndex, req *ExploreRequest, result *ExploreResult) {
	entry, ok := idx[req.Target]
	if !ok {
		return
	}
	result.Files[req.Target] = &FileContent{
		Path:     entry.Path,
		Language: entry.Language,
		Content:  buildFileContent(entry),
	}
}

func (a *ExploreAgent) resolveReadFunction(idx store.FileIndex, req *ExploreRequest, result *ExploreResult) {
	for _, entry := range idx {
		for _, fn := range entry.Functions {
			if store.FuncID(fn) == req.Target {
				result.Functions[req.Target] = &FuncContent{
					FuncID:    req.Target,
					Path:      fn.Path,
					Src:       fn.Src,
					Signature: fn.Signature,
				}
				return
			}
		}
	}
}

func buildFileContent(entry *store.FileEntry) string {
	var sb strings.Builder
	for _, fn := range entry.Functions {
		sb.WriteString(fn.Src)
		sb.WriteString("\n\n")
	}
	for _, typ := range entry.Types {
		sb.WriteString(typ.Src)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
