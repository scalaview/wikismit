package store

import "path/filepath"

func WriteFileIndex(dir string, idx FileIndex) error {
	return writeJSON(filepath.Join(dir, "file_index.json"), idx)
}

func ReadFileIndex(dir string) (FileIndex, error) {
	var idx FileIndex
	err := readJSON(filepath.Join(dir, "file_index.json"), &idx)
	return idx, err
}

func WriteDepGraph(dir string, graph DepGraph) error {
	return writeJSON(filepath.Join(dir, "dep_graph.json"), graph)
}

func ReadDepGraph(dir string) (DepGraph, error) {
	var graph DepGraph
	err := readJSON(filepath.Join(dir, "dep_graph.json"), &graph)
	return graph, err
}

func WriteNavPlan(dir string, plan NavPlan) error {
	return writeJSON(filepath.Join(dir, "nav_plan.json"), plan)
}

func ReadNavPlan(dir string) (NavPlan, error) {
	var plan NavPlan
	err := readJSON(filepath.Join(dir, "nav_plan.json"), &plan)
	return plan, err
}

func WriteSharedContext(dir string, shared SharedContext) error {
	return writeJSON(filepath.Join(dir, "shared_context.json"), shared)
}

func ReadSharedContext(dir string) (SharedContext, error) {
	var shared SharedContext
	err := readJSON(filepath.Join(dir, "shared_context.json"), &shared)
	return shared, err
}
