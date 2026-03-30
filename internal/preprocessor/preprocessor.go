package preprocessor

import (
	"context"
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

func topoSortComponents(graph map[string][]string) [][]string {
	if len(graph) == 0 {
		return [][]string{}
	}

	index := 0
	indices := make(map[string]int, len(graph))
	lowLink := make(map[string]int, len(graph))
	onStack := make(map[string]bool, len(graph))
	stack := make([]string, 0, len(graph))
	components := make([][]string, 0, len(graph))

	var strongConnect func(string)
	strongConnect = func(node string) {
		indices[node] = index
		lowLink[node] = index
		index++
		stack = append(stack, node)
		onStack[node] = true

		for _, dependency := range graph[node] {
			if _, seen := indices[dependency]; !seen {
				strongConnect(dependency)
				if lowLink[dependency] < lowLink[node] {
					lowLink[node] = lowLink[dependency]
				}
				continue
			}
			if onStack[dependency] && indices[dependency] < lowLink[node] {
				lowLink[node] = indices[dependency]
			}
		}

		if lowLink[node] != indices[node] {
			return
		}

		component := make([]string, 0, 1)
		for {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			component = append(component, member)
			if member == node {
				break
			}
		}
		sort.Strings(component)
		components = append(components, component)
	}

	nodes := make([]string, 0, len(graph))
	for node := range graph {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		if _, seen := indices[node]; seen {
			continue
		}
		strongConnect(node)
	}

	componentByNode := make(map[string]int, len(graph))
	for componentIndex, component := range components {
		for _, node := range component {
			componentByNode[node] = componentIndex
		}
	}

	componentGraph := make(map[int][]int, len(components))
	componentInDegree := make(map[int]int, len(components))
	for componentIndex := range components {
		componentGraph[componentIndex] = []int{}
		componentInDegree[componentIndex] = 0
	}

	seenEdges := make(map[[2]int]bool)
	for from, dependencies := range graph {
		fromComponent := componentByNode[from]
		for _, dependency := range dependencies {
			toComponent := componentByNode[dependency]
			if fromComponent == toComponent {
				continue
			}
			edge := [2]int{toComponent, fromComponent}
			if seenEdges[edge] {
				continue
			}
			seenEdges[edge] = true
			componentGraph[toComponent] = append(componentGraph[toComponent], fromComponent)
			componentInDegree[fromComponent]++
		}
	}

	ready := make([]int, 0, len(components))
	for componentIndex, degree := range componentInDegree {
		if degree == 0 {
			ready = append(ready, componentIndex)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		left := components[ready[i]]
		right := components[ready[j]]
		return left[0] < right[0]
	})

	ordered := make([][]string, 0, len(components))
	for len(ready) > 0 {
		componentIndex := ready[0]
		ready = ready[1:]
		ordered = append(ordered, components[componentIndex])

		for _, dependent := range componentGraph[componentIndex] {
			componentInDegree[dependent]--
			if componentInDegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.Slice(ready, func(i, j int) bool {
					left := components[ready[i]]
					right := components[ready[j]]
					return left[0] < right[0]
				})
			}
		}
	}

	return ordered
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
	components := topoSortComponents(sharedGraph)
	if len(components) == 0 {
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

	sharedCtx := make(store.SharedContext, len(sharedGraph))
	for _, component := range components {
		componentSet := make(map[string]bool, len(component))
		for _, moduleID := range component {
			componentSet[moduleID] = true
		}

		if affectedSet != nil {
			for _, moduleID := range component {
				if affectedSet[moduleID] {
					continue
				}
				if summary, ok := existing[moduleID]; ok {
					sharedCtx[moduleID] = summary
				}
			}
		}

		for _, moduleID := range component {
			if affectedSet != nil && !affectedSet[moduleID] {
				if _, ok := sharedCtx[moduleID]; ok {
					continue
				}
			}

			files := moduleFiles[moduleID]
			skeleton := planner.BuildSkeleton(files, idx, cfg.Agent.SkeletonMaxTokens)
			directDeps := make(store.SharedContext, len(sharedGraph[moduleID]))
			for _, dependencyID := range sharedGraph[moduleID] {
				if componentSet[dependencyID] {
					if affectedSet != nil && !affectedSet[dependencyID] {
						if summary, ok := sharedCtx[dependencyID]; ok {
							directDeps[dependencyID] = summary
						}
					}
					continue
				}
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
			if err := llm.ParseJSON(response, &summary); err != nil {
				return nil, err
			}
			sharedCtx[moduleID] = groundSharedSummaryRefs(summary, files, idx)
			if err := writeSharedModuleMarkdown(cfg.ArtifactsDir, moduleID, sharedCtx[moduleID]); err != nil {
				return nil, fmt.Errorf("write shared module markdown for %s: %w", moduleID, err)
			}
		}
	}

	if err := store.WriteSharedContext(cfg.ArtifactsDir, sharedCtx); err != nil {
		return nil, err
	}
	return sharedCtx, nil
}
