package preprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/planner"
	"github.com/scalaview/wikismit/pkg/store"
)

func sharedSubgraph(plan *store.NavPlan, graph store.DepGraph) map[string][]string {
	fileToModule := make(map[string]string)
	sharedModules := make(map[string]bool)
	for _, module := range plan.Modules {
		if !module.Shared {
			continue
		}
		sharedModules[module.ID] = true
		for _, file := range module.Files {
			fileToModule[file] = module.ID
		}
	}

	adjacency := make(map[string][]string, len(sharedModules))
	for moduleID := range sharedModules {
		adjacency[moduleID] = []string{}
	}

	for fromFile, edges := range graph {
		fromModule, ok := fileToModule[fromFile]
		if !ok {
			continue
		}

		seen := make(map[string]bool)
		for _, toFile := range edges {
			toModule, ok := fileToModule[toFile]
			if !ok || toModule == fromModule || seen[toModule] {
				continue
			}
			seen[toModule] = true
			adjacency[fromModule] = append(adjacency[fromModule], toModule)
		}
		sort.Strings(adjacency[fromModule])
	}

	return adjacency
}

func topoSort(graph map[string][]string) ([]string, error) {
	if len(graph) == 0 {
		return []string{}, nil
	}

	inDegree := make(map[string]int, len(graph))
	for node := range graph {
		inDegree[node] = 0
	}
	reverse := make(map[string][]string, len(graph))
	for node := range graph {
		reverse[node] = []string{}
	}
	for node, dependencies := range graph {
		inDegree[node] = len(dependencies)
		for _, dependency := range dependencies {
			reverse[dependency] = append(reverse[dependency], node)
		}
	}

	ready := make([]string, 0, len(inDegree))
	for node, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)

	ordered := make([]string, 0, len(graph))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		ordered = append(ordered, node)

		for _, dependent := range reverse[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.Strings(ready)
			}
		}
	}

	if len(ordered) != len(graph) {
		remaining := make([]string, 0, len(graph)-len(ordered))
		for node, degree := range inDegree {
			if degree > 0 {
				remaining = append(remaining, node)
			}
		}
		sort.Strings(remaining)
		return nil, fmt.Errorf("cycle detected among shared modules: %v", remaining)
	}

	return ordered, nil
}

func RunPreprocessor(ctx context.Context, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (store.SharedContext, error) {
	return runPreprocessor(ctx, nil, plan, idx, graph, cfg, client)
}

func RunPreprocessorFor(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (store.SharedContext, error) {
	affectedSet := make(map[string]bool, len(affected))
	for _, module := range affected {
		if module.Owner != "shared_preprocessor" && !module.Shared {
			continue
		}
		affectedSet[module.ID] = true
	}
	return runPreprocessor(ctx, affectedSet, plan, idx, graph, cfg, client)
}

func runPreprocessor(ctx context.Context, affectedSet map[string]bool, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (store.SharedContext, error) {
	model := cfg.LLM.PreprocessorModel
	if model == "" {
		model = cfg.LLM.PlannerModel
	}

	sharedGraph := sharedSubgraph(plan, graph)
	order, err := topoSort(sharedGraph)
	if err != nil {
		return nil, err
	}
	if len(order) == 0 {
		return store.SharedContext{}, nil
	}

	moduleFiles := make(map[string][]string, len(plan.Modules))
	for _, module := range plan.Modules {
		moduleFiles[module.ID] = append([]string(nil), module.Files...)
	}

	existing := store.SharedContext{}
	if affectedSet != nil {
		loaded, err := store.ReadSharedContext(cfg.ArtifactsDir)
		if err != nil && err != store.ErrArtifactNotFound {
			return nil, err
		}
		if err == nil {
			existing = loaded
		}
	}

	sharedCtx := make(store.SharedContext, len(order))
	for _, moduleID := range order {
		if affectedSet != nil && !affectedSet[moduleID] {
			if summary, ok := existing[moduleID]; ok {
				sharedCtx[moduleID] = summary
				continue
			}
		}

		files := moduleFiles[moduleID]
		skeleton := planner.BuildSkeleton(files, idx, cfg.Agent.SkeletonMaxTokens)
		directDeps := make(store.SharedContext, len(sharedGraph[moduleID]))
		for _, dependencyID := range sharedGraph[moduleID] {
			if summary, ok := sharedCtx[dependencyID]; ok {
				directDeps[dependencyID] = summary
			}
		}
		prompt := buildSharedPrompt(moduleID, skeleton, directDeps)
		response, err := client.Complete(ctx, llm.CompletionRequest{
			Model:       model,
			UserMsg:     prompt,
			MaxTokens:   cfg.LLM.MaxTokens,
			Temperature: cfg.LLM.Temperature,
		})
		if err != nil {
			return nil, err
		}

		var summary store.SharedSummary
		if err := json.Unmarshal([]byte(response), &summary); err != nil {
			return nil, err
		}
		sharedCtx[moduleID] = groundSharedSummaryRefs(summary, files, idx)
	}

	if err := store.WriteSharedContext(cfg.ArtifactsDir, sharedCtx); err != nil {
		return nil, err
	}
	return sharedCtx, nil
}
