package analyzer

import (
	"sort"

	"github.com/scalaview/wikismit/pkg/gitdiff"
	"github.com/scalaview/wikismit/pkg/store"
)

func owningModules(changedFiles []gitdiff.FileChange, plan *store.NavPlan) []string {
	if plan == nil || len(changedFiles) == 0 {
		return []string{}
	}

	fileOwners := make(map[string]string)
	for _, module := range plan.Modules {
		for _, file := range module.Files {
			fileOwners[file] = module.ID
		}
	}

	seen := make(map[string]bool)
	owners := make([]string, 0, len(changedFiles))
	for _, change := range changedFiles {
		moduleID, ok := fileOwners[change.Path]
		if !ok || seen[moduleID] {
			continue
		}
		seen[moduleID] = true
		owners = append(owners, moduleID)
	}

	sort.Strings(owners)
	return owners
}

func buildReverseGraph(graph store.DepGraph) store.DepGraph {
	reversed := make(store.DepGraph, len(graph))
	for from := range graph {
		reversed[from] = []string{}
	}

	for from, edges := range graph {
		for _, to := range edges {
			reversed[to] = append(reversed[to], from)
		}
	}

	for node := range reversed {
		sort.Strings(reversed[node])
	}

	return reversed
}

func ComputeAffected(changedFiles []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module {
	if plan == nil || len(changedFiles) == 0 {
		return []store.Module{}
	}

	moduleByID := make(map[string]store.Module, len(plan.Modules))
	fileOwners := make(map[string]string)
	for _, module := range plan.Modules {
		moduleByID[module.ID] = module
		for _, file := range module.Files {
			fileOwners[file] = module.ID
		}
	}

	affectedModuleIDs := make(map[string]bool)
	queue := make([]string, 0, len(changedFiles))
	seenFiles := make(map[string]bool)

	for _, moduleID := range owningModules(changedFiles, plan) {
		affectedModuleIDs[moduleID] = true
		for _, file := range moduleByID[moduleID].Files {
			if seenFiles[file] {
				continue
			}
			seenFiles[file] = true
			queue = append(queue, file)
		}
	}

	reverse := buildReverseGraph(graph)
	for len(queue) > 0 {
		file := queue[0]
		queue = queue[1:]

		for _, dependent := range reverse[file] {
			if seenFiles[dependent] {
				continue
			}
			seenFiles[dependent] = true
			queue = append(queue, dependent)

			moduleID, ok := fileOwners[dependent]
			if ok {
				affectedModuleIDs[moduleID] = true
			}
		}
	}

	modules := make([]store.Module, 0, len(affectedModuleIDs))
	for moduleID := range affectedModuleIDs {
		module, ok := moduleByID[moduleID]
		if !ok {
			continue
		}
		modules = append(modules, module)
	}

	sort.Slice(modules, func(i int, j int) bool {
		return modules[i].ID < modules[j].ID
	})

	return modules
}
