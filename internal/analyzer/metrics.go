package analyzer

import (
	"math"

	"github.com/scalaview/wikismit/internal/analyzer/lang"
	"github.com/scalaview/wikismit/pkg/store"
)

// ScoringStrategy computes ImportanceScore from raw metrics.
type ScoringStrategy interface {
	Score(m *store.FunctionMetrics, ctx ScoringContext) float64
}

// ScoringContext provides normalization basins for scoring.
type ScoringContext struct {
	TotalFunctions int
	MaxReachable   int
	MaxDepth       int
}

// WeightedScoring implements the multi-dimensional weighted formula.
type WeightedScoring struct {
	DepthWeight     float64
	ReachableWeight float64
	EntryBonus      float64
	ExportedBonus   float64
	LoCWeight       float64
}

func DefaultWeightedScoring() *WeightedScoring {
	return &WeightedScoring{
		DepthWeight:     0.30,
		ReachableWeight: 0.25,
		EntryBonus:      0.20,
		ExportedBonus:   0.10,
		LoCWeight:       0.15,
	}
}

func (s *WeightedScoring) Score(m *store.FunctionMetrics, ctx ScoringContext) float64 {
	depthScore := 0.0
	if m.DepthFromEntryPoint >= 0 && ctx.MaxDepth > 0 {
		depthScore = 1.0 / float64(m.DepthFromEntryPoint+1)
	}

	reachableScore := 0.0
	if ctx.MaxReachable > 0 {
		reachableScore = float64(m.ReachableFromEntry) / float64(ctx.MaxReachable)
	}

	locScore := math.Min(float64(m.LinesOfCode), 100.0) / 100.0

	score := depthScore*s.DepthWeight +
		reachableScore*s.ReachableWeight +
		boolToFloat(m.IsEntryPoint)*s.EntryBonus +
		boolToFloat(m.IsExported)*s.ExportedBonus +
		locScore*s.LoCWeight

	return math.Min(score, 1.0)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// MetricsComputer orchestrates metric extraction and scoring.
type MetricsComputer struct {
	scoring    ScoringStrategy
	complexity lang.ComplexityExtractor
}

func NewMetricsComputer() *MetricsComputer {
	return &MetricsComputer{
		scoring:    DefaultWeightedScoring(),
		complexity: &lang.LinesOfCodeExtractor{},
	}
}

func (mc *MetricsComputer) Compute(
	idx store.FileIndex,
	graph store.CallGraph,
) store.MetricsMap {
	outDegree := mc.computeOutDegree(idx)
	entryPoints := mc.computeEntryPoints(idx, graph)
	depths := mc.computeEntryPointDepths(graph, entryPoints)
	reachable := mc.computeReachableFromEntry(graph, entryPoints)
	loc := mc.computeLinesOfCode(idx)

	metrics := make(store.MetricsMap)

	for _, entry := range idx {
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			if id == "" {
				continue
			}
			depth, depthFound := depths[id]
			if !depthFound {
				depth = -1
			}
			m := &store.FunctionMetrics{
				FuncID:              id,
				OutDegree:           outDegree[id],
				DepthFromEntryPoint: depth,
				ReachableFromEntry:  reachable[id],
				IsExported:          fn.Exported,
				IsEntryPoint:        entryPoints[id],
				LinesOfCode:         loc[id],
			}
			metrics[id] = m
		}
	}

	ctx := mc.buildScoringContext(metrics)
	for _, m := range metrics {
		m.ImportanceScore = mc.scoring.Score(m, ctx)
	}

	return metrics
}

func (mc *MetricsComputer) computeOutDegree(idx store.FileIndex) map[string]int {
	result := make(map[string]int)
	for _, entry := range idx {
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			count := 0
			for _, call := range fn.Calls {
				if call.Ownership == store.OwnershipInternal {
					count++
				}
			}
			result[id] = count
		}
	}
	return result
}

func (mc *MetricsComputer) computeEntryPoints(idx store.FileIndex, graph store.CallGraph) map[string]bool {
	callers := make(map[string]struct{})
	for _, callees := range graph {
		for _, callee := range callees {
			callers[callee] = struct{}{}
		}
	}

	result := make(map[string]bool)
	for _, entry := range idx {
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			if fn.Exported {
				if _, hasCaller := callers[id]; !hasCaller {
					result[id] = true
				}
			}
		}
	}
	return result
}

func (mc *MetricsComputer) computeEntryPointDepths(graph store.CallGraph, entryPoints map[string]bool) map[string]int {
	depths := make(map[string]int)
	queue := make([]string, 0, len(entryPoints))

	for id := range entryPoints {
		depths[id] = 0
		queue = append(queue, id)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentDepth := depths[current]

		for _, callee := range graph[current] {
			if _, visited := depths[callee]; !visited {
				depths[callee] = currentDepth + 1
				queue = append(queue, callee)
			}
		}
	}

	// Mark unreachable functions from CallGraph
	for caller := range graph {
		if _, ok := depths[caller]; !ok {
			depths[caller] = -1
		}
		for _, callee := range graph[caller] {
			if _, ok := depths[callee]; !ok {
				depths[callee] = -1
			}
		}
	}

	return depths
}

func (mc *MetricsComputer) computeReachableFromEntry(graph store.CallGraph, entryPoints map[string]bool) map[string]int {
	// Topological sort using Kahn's algorithm
	inDegree := make(map[string]int)
	for caller := range graph {
		if _, ok := inDegree[caller]; !ok {
			inDegree[caller] = 0
		}
		for _, callee := range graph[caller] {
			inDegree[callee]++
		}
	}

	reachable := make(map[string]int)
	queue := make([]string, 0)
	for id := range entryPoints {
		reachable[id] = 1
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, callee := range graph[current] {
			reachable[callee] += reachable[current]
			inDegree[callee]--
			if inDegree[callee] == 0 {
				queue = append(queue, callee)
			}
		}
	}

	return reachable
}

func (mc *MetricsComputer) computeLinesOfCode(idx store.FileIndex) map[string]int {
	result := make(map[string]int)
	for _, entry := range idx {
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			result[id] = mc.complexity.LinesOfCode(fn.Src)
		}
	}
	return result
}

func (mc *MetricsComputer) buildScoringContext(metrics store.MetricsMap) ScoringContext {
	ctx := ScoringContext{TotalFunctions: len(metrics)}
	for _, m := range metrics {
		if m.ReachableFromEntry > ctx.MaxReachable {
			ctx.MaxReachable = m.ReachableFromEntry
		}
		if m.DepthFromEntryPoint > ctx.MaxDepth {
			ctx.MaxDepth = m.DepthFromEntryPoint
		}
	}
	return ctx
}
