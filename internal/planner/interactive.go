package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func RunInteractivePlanner(ctx context.Context, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (*store.NavPlan, error) {
	_ = graph

	interactiveCfg := cfg.Analysis.InteractivePlanner
	if interactiveCfg == nil {
		return nil, fmt.Errorf("interactive planner config is required")
	}

	callGraph, err := store.ReadCallGraph(cfg.ArtifactsDir)
	if err != nil {
		return nil, fmt.Errorf("read call graph: %w", err)
	}
	eventIdx, err := store.ReadEventFactIndex(cfg.ArtifactsDir)
	if err != nil {
		return nil, fmt.Errorf("read event fact index: %w", err)
	}

	skeleton := BuildPlannerSkeleton(idx, cfg.Agent.SkeletonMaxTokens)
	requestLimit := interactiveCfg.MaxRequestsPerRound
	if requestLimit <= 0 {
		requestLimit = 5
	}
	maxRounds := interactiveCfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 4
	}

	contextState := &PlannerRoundContext{
		Skeleton: skeleton,
	}

	for round := 1; round <= maxRounds; round++ {
		contextState.Round = round
		response, err := client.Complete(ctx, &llm.CompletionRequest{
			Model:       cfg.LLM.PlannerModel,
			SystemMsg:   "You are an interactive planner.",
			UserMsg:     buildInteractivePlannerPrompt(contextState, cfg.Analysis.SharedModuleThreshold),
			MaxTokens:   cfg.LLM.MaxTokens,
			Temperature: cfg.LLM.Temperature,
		})
		if err != nil {
			return nil, err
		}

		var roundRequest PlannerRoundRequest
		if err := llm.ParseJSON(response, &roundRequest); err != nil {
			return nil, fmt.Errorf("parse interactive round %d: %w", round, err)
		}

		contextState.ExplorationContext = roundRequest.Understanding

		if roundRequest.Navigation != nil {
			plan, err := parseInteractiveNavigation(*roundRequest.Navigation, idx)
			if err != nil {
				return nil, err
			}
			stampNavPlanMetadata(plan)
			return plan, nil
		}

		if len(roundRequest.Requests) == 0 {
			return nil, fmt.Errorf("interactive round %d returned no requests or navigation", round)
		}

		requests := roundRequest.Requests
		if len(requests) > requestLimit {
			requests = append([]*PlannerRequest(nil), requests[:requestLimit]...)
			contextState.ExplorationContext = appendRoundNote(contextState.ExplorationContext, fmt.Sprintf("round %d truncated %d request(s)", round, len(roundRequest.Requests)-len(requests)))
		}

		responses, err := routePlannerRequests(idx, callGraph, eventIdx, requests)
		if err != nil {
			return nil, err
		}
		contextState.PreviousResponses = append(contextState.PreviousResponses, responses...)
	}

	return nil, fmt.Errorf("interactive planner exhausted max rounds (%d) without terminal navigation", maxRounds)
}

func routePlannerRequests(idx store.FileIndex, callGraph store.CallGraph, eventIdx store.EventFactIndex, requests []*PlannerRequest) ([]*PlannerResponseEnvelope, error) {
	responses := make([]*PlannerResponseEnvelope, 0, len(requests))
	for _, request := range requests {
		if request == nil {
			continue
		}
		envelope, err := routePlannerRequest(idx, callGraph, eventIdx, request)
		if err != nil {
			return nil, err
		}
		responses = append(responses, envelope)
	}
	return responses, nil
}

func routePlannerRequest(idx store.FileIndex, callGraph store.CallGraph, eventIdx store.EventFactIndex, request *PlannerRequest) (*PlannerResponseEnvelope, error) {
	switch request.Type {
	case "read_file":
		var params plannerReadFileParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, fmt.Errorf("decode read_file params: %w", err)
		}
		entry := idx[params.Target]
		if entry == nil {
			return &PlannerResponseEnvelope{Type: request.Type, Result: &plannerReadFileResult{Target: params.Target}}, nil
		}
		return &PlannerResponseEnvelope{Type: request.Type, Result: &plannerReadFileResult{Target: params.Target, Content: buildInteractiveFileContent(entry)}}, nil
	case "read_function":
		var params plannerReadFunctionParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, fmt.Errorf("decode read_function params: %w", err)
		}
		for _, entry := range idx {
			for _, fn := range entry.Functions {
				if store.FuncID(fn) == params.Target {
					return &PlannerResponseEnvelope{Type: request.Type, Result: &plannerReadFunctionResult{Target: params.Target, Signature: fn.Signature, Src: fn.Src}}, nil
				}
			}
		}
		return &PlannerResponseEnvelope{Type: request.Type, Result: &plannerReadFunctionResult{Target: params.Target}}, nil
	case "call_chain":
		var params CallChainQuery
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, fmt.Errorf("decode call_chain params: %w", err)
		}
		result, err := QueryCallChain(idx, callGraph, eventIdx, params)
		if err != nil {
			return nil, fmt.Errorf("route call_chain: %w", err)
		}
		return &PlannerResponseEnvelope{Type: request.Type, Result: result}, nil
	case "event_flow":
		var params EventFlowQuery
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, fmt.Errorf("decode event_flow params: %w", err)
		}
		result, err := QueryEventFlow(idx, callGraph, eventIdx, params)
		if err != nil {
			return nil, fmt.Errorf("route event_flow: %w", err)
		}
		return &PlannerResponseEnvelope{Type: request.Type, Result: result}, nil
	default:
		return nil, fmt.Errorf("unknown request type %q", request.Type)
	}
}

func parseInteractiveNavigation(raw json.RawMessage, idx store.FileIndex) (*store.NavPlan, error) {
	var plan store.NavPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, fmt.Errorf("parse navigation: %w", err)
	}
	if err := validateNavPlan(plan, idx); err != nil {
		return nil, fmt.Errorf("validate navigation: %w", err)
	}
	return &plan, nil
}
func appendRoundNote(existing string, note string) string {
	if note == "" {
		return existing
	}
	if existing == "" {
		return note
	}
	return existing + "\n" + note
}

type plannerReadFileParams struct {
	Target string `json:"target"`
}

type plannerReadFunctionParams struct {
	Target string `json:"target"`
}

type plannerReadFileResult struct {
	Target  string `json:"target"`
	Content string `json:"content,omitempty"`
}

type plannerReadFunctionResult struct {
	Target    string `json:"target"`
	Signature string `json:"signature,omitempty"`
	Src       string `json:"src,omitempty"`
}

func buildInteractiveFileContent(entry *store.FileEntry) string {
	if entry == nil {
		return ""
	}

	var sb strings.Builder
	functions := append([]*store.FunctionDecl(nil), entry.Functions...)
	sort.Slice(functions, func(i int, j int) bool {
		left := functions[i]
		right := functions[j]
		if left == nil || right == nil {
			return right == nil
		}
		return store.FuncID(left) < store.FuncID(right)
	})
	for _, fn := range functions {
		if fn == nil || fn.Src == "" {
			continue
		}
		sb.WriteString(fn.Src)
		sb.WriteString("\n\n")
	}
	for _, typ := range entry.Types {
		if typ == nil || typ.Src == "" {
			continue
		}
		sb.WriteString(typ.Src)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
