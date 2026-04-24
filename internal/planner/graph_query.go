package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/scalaview/wikismit/pkg/store"
)

func QueryCallChain(idx store.FileIndex, callGraph store.CallGraph, eventIdx store.EventFactIndex, q CallChainQuery) (*FlowGraphResult, error) {
	funcByID := buildFunctionIndex(idx)
	funcGraph := normalizeCallGraph(idx, callGraph)

	seedFn := funcByID[q.FunctionRef]
	if seedFn == nil {
		return nil, fmt.Errorf("unknown function ref %q: use exact refs from QUERYABLE_FUNCTIONS for read_function/call_chain, or use read_file with a FILE path for type and file inspection", q.FunctionRef)
	}

	depth := q.Depth
	if depth <= 0 {
		depth = 1
	}

	builder := newFlowGraphBuilder(q.MaxNodes, q.MaxEdges)
	builder.addFunctionNode(seedFn)

	graph := funcGraph
	upstream := false
	if strings.EqualFold(q.Direction, "upstream") {
		graph = reverseCallGraph(funcGraph)
		upstream = true
	}

	type queueItem struct {
		id    string
		depth int
	}
	queue := []queueItem{{id: q.FunctionRef, depth: 0}}
	seen := map[string]int{q.FunctionRef: 0}

	for len(queue) > 0 {
		if builder.truncated {
			break
		}
		current := queue[0]
		queue = queue[1:]
		if current.depth >= depth {
			continue
		}

		nextIDs := append([]string(nil), graph[current.id]...)
		sort.Strings(nextIDs)
		for _, nextID := range nextIDs {
			nextFn := funcByID[nextID]
			if nextFn == nil {
				continue
			}
			if !builder.addFunctionNode(nextFn) {
				continue
			}
			if upstream {
				if !builder.addEdge(nextID, current.id, "call", "confirmed", "call_graph") {
					continue
				}
			} else {
				if !builder.addEdge(current.id, nextID, "call", "confirmed", "call_graph") {
					continue
				}
			}

			nextDepth := current.depth + 1
			if prevDepth, ok := seen[nextID]; ok && prevDepth <= nextDepth {
				continue
			}
			seen[nextID] = nextDepth
			queue = append(queue, queueItem{id: nextID, depth: nextDepth})
		}
	}

	if q.IncludeEvents {
		enrichCallChainWithEvents(builder, eventIdx)
	}

	return builder.result(), nil
}

func QueryEventFlow(idx store.FileIndex, callGraph store.CallGraph, eventIdx store.EventFactIndex, q EventFlowQuery) (*FlowGraphResult, error) {
	funcByID := buildFunctionIndex(idx)
	funcGraph := normalizeCallGraph(idx, callGraph)
	event := findEventEntry(eventIdx, q.EventName)
	if event == nil {
		return nil, fmt.Errorf("unknown event %q", q.EventName)
	}

	builder := newFlowGraphBuilder(q.MaxNodes, q.MaxEdges)
	eventNodeID := eventNodeID(event.EventName)
	builder.addNode(eventNodeID, "event", event.EventName)

	for _, fact := range event.Publishers {
		if fact == nil {
			continue
		}
		fn := funcByID[fact.FuncID]
		if !builder.addFunctionNode(fn) {
			continue
		}
		builder.addEdge(fact.FuncID, eventNodeID, "publish", "confirmed", "event_fact")
		if q.ExpandPublishers {
			expandCallGraph(builder, funcByID, funcGraph, fact.FuncID, 1)
		}
	}

	for _, fact := range event.Handlers {
		if fact == nil {
			continue
		}
		fn := funcByID[fact.FuncID]
		if !builder.addFunctionNode(fn) {
			continue
		}
		builder.addEdge(eventNodeID, fact.FuncID, "handle", "confirmed", "event_fact")
		if q.ExpandHandlers {
			expandCallGraph(builder, funcByID, funcGraph, fact.FuncID, q.HandlerDepth)
		}
	}

	for _, fact := range event.Registrations {
		if fact == nil {
			continue
		}
		fn := funcByID[fact.FuncID]
		if !builder.addFunctionNode(fn) {
			continue
		}
		if fact.HandlerRef != "" {
			if handlerFn := funcByID[fact.HandlerRef]; builder.addFunctionNode(handlerFn) {
				builder.addEdge(fact.FuncID, fact.HandlerRef, "register", "confirmed", "event_fact")
			}
		}
		builder.addCandidateRead(fact.FuncID, fmt.Sprintf("registration for event %s", event.EventName))
	}

	if len(event.Publishers) > 0 && len(event.Handlers) == 0 {
		builder.addMissingLink(eventNodeID, "", "no confirmed handlers")
	}

	return builder.result(), nil
}

func reverseCallGraph(callGraph store.CallGraph) store.CallGraph {
	reverse := make(store.CallGraph)

	callers := make([]string, 0, len(callGraph))
	for caller := range callGraph {
		callers = append(callers, caller)
		if _, ok := reverse[caller]; !ok {
			reverse[caller] = []string{}
		}
	}
	sort.Strings(callers)

	for _, caller := range callers {
		callees := append([]string(nil), callGraph[caller]...)
		sort.Strings(callees)
		for _, callee := range callees {
			reverse[callee] = append(reverse[callee], caller)
		}
	}

	for node := range reverse {
		sort.Strings(reverse[node])
	}

	return reverse
}

func normalizeCallGraph(idx store.FileIndex, callGraph store.CallGraph) store.CallGraph {
	targetToFuncID := make(map[string]string)
	for _, entry := range idx {
		if entry == nil {
			continue
		}
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			funcID := store.FuncID(fn)
			if funcID == "" {
				continue
			}
			targetToFuncID[fn.Path+"#"+fn.Name] = funcID
		}
	}

	normalized := make(store.CallGraph)
	callers := make([]string, 0, len(callGraph))
	for caller := range callGraph {
		callers = append(callers, caller)
	}
	sort.Strings(callers)

	for _, caller := range callers {
		callerID := targetToFuncID[caller]
		if callerID == "" {
			callerID = caller
		}
		callees := append([]string(nil), callGraph[caller]...)
		sort.Strings(callees)
		translated := make([]string, 0, len(callees))
		for _, callee := range callees {
			calleeID := targetToFuncID[callee]
			if calleeID == "" {
				calleeID = callee
			}
			translated = append(translated, calleeID)
		}
		if len(translated) > 0 {
			normalized[callerID] = translated
		}
	}

	return normalized
}

func buildFunctionIndex(idx store.FileIndex) map[string]*store.FunctionDecl {
	funcByID := make(map[string]*store.FunctionDecl)
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
			funcByID[id] = fn
		}
	}
	return funcByID
}

type flowGraphBuilder struct {
	nodeByID     map[string]*FlowNode
	edgeByKey    map[string]*FlowEdge
	missingByKey map[string]*MissingLink
	readByKey    map[string]*CandidateRead
	maxNodes     int
	maxEdges     int
	truncated    bool
}

func newFlowGraphBuilder(maxNodes int, maxEdges int) *flowGraphBuilder {
	return &flowGraphBuilder{
		nodeByID:     make(map[string]*FlowNode),
		edgeByKey:    make(map[string]*FlowEdge),
		missingByKey: make(map[string]*MissingLink),
		readByKey:    make(map[string]*CandidateRead),
		maxNodes:     maxNodes,
		maxEdges:     maxEdges,
	}
}

func (b *flowGraphBuilder) addFunctionNode(fn *store.FunctionDecl) bool {
	if fn == nil {
		return false
	}
	id := store.FuncID(fn)
	if id == "" {
		return false
	}
	return b.addNode(id, "function", fn.Name)
}

func (b *flowGraphBuilder) addNode(id string, kind string, label string) bool {
	if id == "" {
		return false
	}
	if _, exists := b.nodeByID[id]; exists {
		return true
	}
	if b.maxNodes > 0 && len(b.nodeByID) >= b.maxNodes {
		b.truncated = true
		return false
	}
	b.nodeByID[id] = &FlowNode{ID: id, Kind: kind, Label: label}
	return true
}

func (b *flowGraphBuilder) addEdge(from string, to string, kind string, confidence string, source string) bool {
	if from == "" || to == "" || kind == "" {
		return false
	}
	key := edgeKey(from, to, kind)
	if _, exists := b.edgeByKey[key]; exists {
		return true
	}
	if b.maxEdges > 0 && len(b.edgeByKey) >= b.maxEdges {
		b.truncated = true
		return false
	}
	b.edgeByKey[key] = &FlowEdge{
		From:       from,
		To:         to,
		Kind:       kind,
		Confidence: confidence,
		Source:     source,
	}
	return true
}

func (b *flowGraphBuilder) addMissingLink(from string, to string, problem string) {
	key := from + "\x00" + to + "\x00" + problem
	if _, exists := b.missingByKey[key]; exists {
		return
	}
	b.missingByKey[key] = &MissingLink{From: from, To: to, Problem: problem}
}

func (b *flowGraphBuilder) addCandidateRead(target string, reason string) {
	key := target + "\x00" + reason
	if _, exists := b.readByKey[key]; exists {
		return
	}
	b.readByKey[key] = &CandidateRead{Target: target, Reason: reason}
}

func (b *flowGraphBuilder) result() *FlowGraphResult {
	return &FlowGraphResult{
		Nodes:          collectSortedNodes(b.nodeByID),
		Edges:          collectSortedEdges(b.edgeByKey),
		MissingLinks:   collectSortedMissingLinks(b.missingByKey),
		CandidateReads: collectSortedCandidateReads(b.readByKey),
		Truncated:      b.truncated,
	}
}

func enrichCallChainWithEvents(builder *flowGraphBuilder, eventIdx store.EventFactIndex) {
	functionIDs := make([]string, 0, len(builder.nodeByID))
	for id, node := range builder.nodeByID {
		if node != nil && node.Kind == "function" {
			functionIDs = append(functionIDs, id)
		}
	}
	sort.Strings(functionIDs)

	for _, event := range sortedEventEntries(eventIdx.Events) {
		if event == nil {
			continue
		}
		eventNodeID := eventNodeID(event.EventName)

		for _, fact := range event.Publishers {
			if fact == nil || !containsString(functionIDs, fact.FuncID) {
				continue
			}
			if builder.addNode(eventNodeID, "event", event.EventName) {
				builder.addEdge(fact.FuncID, eventNodeID, "publish", "confirmed", "event_fact")
			}
		}
		for _, fact := range event.Handlers {
			if fact == nil || !containsString(functionIDs, fact.FuncID) {
				continue
			}
			if builder.addNode(eventNodeID, "event", event.EventName) {
				builder.addEdge(eventNodeID, fact.FuncID, "handle", "confirmed", "event_fact")
			}
		}
		for _, fact := range event.Registrations {
			if fact == nil || !containsString(functionIDs, fact.FuncID) || fact.HandlerRef == "" {
				continue
			}
			if _, ok := builder.nodeByID[fact.HandlerRef]; !ok {
				continue
			}
			builder.addEdge(fact.FuncID, fact.HandlerRef, "register", "confirmed", "event_fact")
		}
	}
}

func expandCallGraph(builder *flowGraphBuilder, funcByID map[string]*store.FunctionDecl, callGraph store.CallGraph, seed string, depth int) {
	if depth <= 0 {
		return
	}

	type queueItem struct {
		id    string
		depth int
	}
	queue := []queueItem{{id: seed, depth: 0}}
	seen := map[string]int{seed: 0}

	for len(queue) > 0 {
		if builder.truncated {
			break
		}
		current := queue[0]
		queue = queue[1:]
		if current.depth >= depth {
			continue
		}
		nextIDs := append([]string(nil), callGraph[current.id]...)
		sort.Strings(nextIDs)
		for _, nextID := range nextIDs {
			nextFn := funcByID[nextID]
			if !builder.addFunctionNode(nextFn) {
				continue
			}
			if !builder.addEdge(current.id, nextID, "call", "confirmed", "call_graph") {
				continue
			}
			nextDepth := current.depth + 1
			if prevDepth, ok := seen[nextID]; ok && prevDepth <= nextDepth {
				continue
			}
			seen[nextID] = nextDepth
			queue = append(queue, queueItem{id: nextID, depth: nextDepth})
		}
	}
}

func findEventEntry(eventIdx store.EventFactIndex, eventName string) *store.EventEntry {
	for _, entry := range sortedEventEntries(eventIdx.Events) {
		if entry != nil && entry.EventName == eventName {
			return entry
		}
	}
	return nil
}

func sortedEventEntries(entries []*store.EventEntry) []*store.EventEntry {
	sorted := append([]*store.EventEntry(nil), entries...)
	sort.Slice(sorted, func(i int, j int) bool {
		left := ""
		if sorted[i] != nil {
			left = sorted[i].EventName
		}
		right := ""
		if sorted[j] != nil {
			right = sorted[j].EventName
		}
		return left < right
	})
	return sorted
}

func eventNodeID(eventName string) string {
	return "event:" + eventName
}

func edgeKey(from string, to string, kind string) string {
	return from + "\x00" + to + "\x00" + kind
}

func containsString(values []string, target string) bool {
	idx := sort.SearchStrings(values, target)
	return idx < len(values) && values[idx] == target
}

func collectSortedNodes(nodeByID map[string]*FlowNode) []*FlowNode {
	ids := make([]string, 0, len(nodeByID))
	for id := range nodeByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	nodes := make([]*FlowNode, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, nodeByID[id])
	}
	return nodes
}

func collectSortedEdges(edgeByKey map[string]*FlowEdge) []*FlowEdge {
	keys := make([]string, 0, len(edgeByKey))
	for key := range edgeByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	edges := make([]*FlowEdge, 0, len(keys))
	for _, key := range keys {
		edges = append(edges, edgeByKey[key])
	}
	return edges
}

func collectSortedMissingLinks(missingByKey map[string]*MissingLink) []*MissingLink {
	keys := make([]string, 0, len(missingByKey))
	for key := range missingByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	links := make([]*MissingLink, 0, len(keys))
	for _, key := range keys {
		links = append(links, missingByKey[key])
	}
	return links
}

func collectSortedCandidateReads(readByKey map[string]*CandidateRead) []*CandidateRead {
	keys := make([]string, 0, len(readByKey))
	for key := range readByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	reads := make([]*CandidateRead, 0, len(keys))
	for _, key := range keys {
		reads = append(reads, readByKey[key])
	}
	return reads
}
