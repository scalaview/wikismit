package analyzer

import (
	"sort"

	"github.com/scalaview/wikismit/pkg/store"
)

// BuildCalledByIndex populates the CalledBy field on each FunctionDecl by
// reversing the resolved call graph. After this call, every function knows
// which callers invoke it, enabling upstream call chain queries.
//
// The function handles the key mismatch between CallGraph keys (path#Name,
// produced by functionTarget in linker.go) and FuncID keys (path#Receiver#Name
// for methods) by building a translation map.
func BuildCalledByIndex(idx store.FileIndex, graph store.CallGraph) {
	// Build translation: CallGraph key (path#Name) → FuncID (path#Receiver#Name for methods)
	targetToFuncID := make(map[string]string)
	funcLookup := make(map[string]*store.FunctionDecl)
	for _, entry := range idx {
		if entry == nil {
			continue
		}
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			if id == "" {
				continue
			}
			funcLookup[id] = fn
			targetKey := fn.Path + "#" + fn.Name
			targetToFuncID[targetKey] = id
		}
	}

	// Walk the call graph in deterministic order and build reverse references
	callerTargets := make([]string, 0, len(graph))
	for callerTarget := range graph {
		callerTargets = append(callerTargets, callerTarget)
	}
	sort.Strings(callerTargets)

	for _, callerTarget := range callerTargets {
		callees := graph[callerTarget]
		callerID := targetToFuncID[callerTarget]
		if callerID == "" {
			callerID = callerTarget
		}
		callerFn := funcLookup[callerID]
		if callerFn == nil {
			continue
		}

		for _, calleeTarget := range callees {
			calleeID := targetToFuncID[calleeTarget]
			if calleeID == "" {
				calleeID = calleeTarget
			}
			calleeFn := funcLookup[calleeID]
			if calleeFn == nil {
				continue
			}
			calleeFn.CalledBy = append(calleeFn.CalledBy, &store.CallRef{
				Name:     callerFn.Name,
				Receiver: callerFn.Receiver,
				Path:     callerFn.Path,
				Line:     callerFn.LineStart,
			})
		}
	}

	// Deduplicate and sort CalledBy for deterministic output
	for _, entry := range idx {
		if entry == nil {
			continue
		}
		for _, fn := range entry.Functions {
			if fn == nil || len(fn.CalledBy) == 0 {
				continue
			}
			fn.CalledBy = dedupAndSortCalledBy(fn.CalledBy)
		}
	}
}

func dedupAndSortCalledBy(refs []*store.CallRef) []*store.CallRef {
	seen := make(map[string]struct{}, len(refs))
	unique := make([]*store.CallRef, 0, len(refs))
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		key := ref.Path + "#" + ref.Receiver + "#" + ref.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, ref)
	}

	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Path != unique[j].Path {
			return unique[i].Path < unique[j].Path
		}
		if unique[i].Name != unique[j].Name {
			return unique[i].Name < unique[j].Name
		}
		return unique[i].Receiver < unique[j].Receiver
	})

	return unique
}
