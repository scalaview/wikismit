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
	if m.DepthFromEntryPoint >= 0 && m.DepthFromEntryPoint < math.MaxInt32 {
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
	// Build FuncID-keyed call graph from FileIndex to avoid key mismatch
	// (linker uses path#Name, FuncID uses path#Receiver#Name for methods).
	funcGraph, targetToFuncID := mc.buildFuncIDGraph(idx, graph)

	outDegree := mc.computeOutDegree(idx)
	entryPoints := mc.computeEntryPoints(idx, graph)
	depths := mc.computeEntryPointDepths(funcGraph, entryPoints)
	reachable := mc.computeReachableFromEntry(funcGraph, entryPoints)
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

	_ = targetToFuncID // used during graph translation
	return metrics
}

// buildFuncIDGraph translates the CallGraph from linker keys (path#Name)
// to FuncID keys (path#Receiver#Name for methods).
func (mc *MetricsComputer) buildFuncIDGraph(
	idx store.FileIndex,
	graph store.CallGraph,
) (funcGraph store.CallGraph, targetToFuncID map[string]string) {
	// Map linker target keys → FuncID keys
	targetToFuncID = make(map[string]string)
	for _, entry := range idx {
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			// Both the linker key and FuncID for regular functions
			targetKey := fn.Path + "#" + fn.Name
			targetToFuncID[targetKey] = id
		}
	}

	funcGraph = make(store.CallGraph)
	for caller, callees := range graph {
		callerID := targetToFuncID[caller]
		if callerID == "" {
			callerID = caller // fallback: unmapped key
		}
		var translated []string
		for _, callee := range callees {
			calleeID := targetToFuncID[callee]
			if calleeID == "" {
				calleeID = callee // fallback
			}
			translated = append(translated, calleeID)
		}
		if len(translated) > 0 {
			funcGraph[callerID] = translated
		}
	}
	return funcGraph, targetToFuncID
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

	// Mark unreachable functions from graph
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

// computeReachableFromEntry counts distinct paths from entry points to each
// function. Uses per-entry-point BFS with a visited set, so cycles are
// handled naturally — each entry point contributes at most 1 to a function's
// reachable count regardless of cycle structure.
func (mc *MetricsComputer) computeReachableFromEntry(graph store.CallGraph, entryPoints map[string]bool) map[string]int {
	reachable := make(map[string]int)

	for ep := range entryPoints {
		visited := map[string]struct{}{ep: {}}
		queue := []string{ep}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			for _, callee := range graph[current] {
				if _, seen := visited[callee]; !seen {
					visited[callee] = struct{}{}
					reachable[callee]++
					queue = append(queue, callee)
				}
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
	}
	return ctx
}
