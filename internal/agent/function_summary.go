package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	promptpkg "github.com/scalaview/wikismit/internal/agent/prompt"
	"github.com/scalaview/wikismit/internal/llm"
	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

type FunctionSummaryConfig struct {
	Model               string
	MaxTokens           int
	ContextBudget       int
	MaxRetries          int
	DependencyDepth     int
	Language            string
	ImportanceThreshold float64
}

type FunctionSummaryAgent struct {
	client llm.Client
	cfg    *FunctionSummaryConfig
	logger logpkg.Logger
}

type FuncSign string

type fnKey struct {
	path     string
	receiver string
	name     string
}

func (f *fnKey) Sign() FuncSign {
	if f == nil {
		return ""
	}
	if f.path == "" && f.name == "" {
		return ""
	}
	if f.receiver == "" {
		return FuncSign(f.path + "#" + f.name)
	}
	return FuncSign(f.path + "#" + f.receiver + "#" + f.name)
}

func newFnKey(fn *store.FunctionDecl) *fnKey {
	if fn == nil {
		return nil
	}
	return &fnKey{path: fn.Path, receiver: fn.Receiver, name: fn.Name}
}

type functionSummaryResult struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

type functionSummaryResponse struct {
	Functions []*functionSummaryResult `json:"functions"`
}

type functionSummaryBatchKey struct {
	path string
	id   string
}

type functionSummaryMatch struct {
	key    *fnKey
	result *functionSummaryResult
}

type FunctionSummaryRunError struct {
	Failed  map[FuncSign]error
	Blocked []FuncSign
}

func (e *FunctionSummaryRunError) Error() string {
	if e == nil || (len(e.Failed) == 0 && len(e.Blocked) == 0) {
		return ""
	}

	failedSigns := make([]FuncSign, 0, len(e.Failed))
	for sign := range e.Failed {
		failedSigns = append(failedSigns, sign)
	}
	sort.Slice(failedSigns, func(i int, j int) bool {
		return failedSigns[i] < failedSigns[j]
	})

	failureParts := make([]string, 0, len(failedSigns))
	for _, sign := range failedSigns {
		if err := e.Failed[sign]; err != nil {
			failureParts = append(failureParts, fmt.Sprintf("%s: %v", sign, err))
			continue
		}
		failureParts = append(failureParts, string(sign))
	}

	blocked := append([]FuncSign(nil), e.Blocked...)
	sort.Slice(blocked, func(i int, j int) bool {
		return blocked[i] < blocked[j]
	})

	blockedParts := make([]string, 0, len(blocked))
	for _, sign := range blocked {
		blockedParts = append(blockedParts, string(sign))
	}

	switch {
	case len(failureParts) == 0:
		return fmt.Sprintf("function summary run blocked: [%s]", strings.Join(blockedParts, ", "))
	case len(blockedParts) == 0:
		return fmt.Sprintf("function summary run completed with %d failures: [%s]", len(failedSigns), strings.Join(failureParts, "; "))
	default:
		return fmt.Sprintf("function summary run completed with %d failures: [%s]; blocked: [%s]", len(failedSigns), strings.Join(failureParts, "; "), strings.Join(blockedParts, ", "))
	}
}

func newFunctionSummaryRunError(failed map[FuncSign]error, blocked []FuncSign) *FunctionSummaryRunError {
	clonedFailed := make(map[FuncSign]error, len(failed))
	for sign, err := range failed {
		if sign == "" || err == nil {
			continue
		}
		clonedFailed[sign] = err
	}

	seenBlocked := make(map[FuncSign]struct{}, len(blocked))
	clonedBlocked := make([]FuncSign, 0, len(blocked))
	for _, sign := range blocked {
		if sign == "" {
			continue
		}
		if _, failed := clonedFailed[sign]; failed {
			continue
		}
		if _, seen := seenBlocked[sign]; seen {
			continue
		}
		seenBlocked[sign] = struct{}{}
		clonedBlocked = append(clonedBlocked, sign)
	}
	sort.Slice(clonedBlocked, func(i int, j int) bool {
		return clonedBlocked[i] < clonedBlocked[j]
	})

	return &FunctionSummaryRunError{Failed: clonedFailed, Blocked: clonedBlocked}
}

func newFunctionSummaryBatchKey(fk *fnKey) functionSummaryBatchKey {
	id := strings.TrimSpace(fk.name)
	if fk.receiver != "" {
		id = strings.TrimSpace(fk.receiver) + "#" + id
	}
	return functionSummaryBatchKey{
		path: strings.TrimSpace(fk.path),
		id:   id,
	}
}

func newFunctionSummaryKey(path, name string) functionSummaryBatchKey {
	return functionSummaryBatchKey{
		path: strings.TrimSpace(path),
		id:   strings.TrimSpace(name),
	}
}

type depGraph struct {
	inDegree map[FuncSign]int
	deps     map[FuncSign][]*fnKey
	reverse  map[FuncSign][]*fnKey
	pending  map[FuncSign]*fnKey
}

func newDepGraph(idx store.FileIndex) *depGraph {
	graph := &depGraph{
		inDegree: make(map[FuncSign]int),
		deps:     make(map[FuncSign][]*fnKey),
		reverse:  make(map[FuncSign][]*fnKey),
		pending:  make(map[FuncSign]*fnKey),
	}

	for _, entry := range idx {
		if entry == nil {
			continue
		}
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			if strings.TrimSpace(fn.Summary) != "" {
				continue
			}
			if strings.TrimSpace(fn.Src) == "" {
				continue
			}

			key := newFnKey(fn)
			sign := key.Sign()
			if sign == "" {
				continue
			}

			graph.pending[sign] = key
			graph.inDegree[sign] = 0
		}
	}

	for sign, key := range graph.pending {
		fn := lookupFunction(idx, key)
		if fn == nil {
			continue
		}

		seenDeps := make(map[FuncSign]struct{})
		for _, call := range fn.Calls {
			if call == nil || call.Ownership != store.OwnershipInternal {
				continue
			}

			calleeKey := resolvedInternalCallKey(idx, call)
			calleeSign := calleeKey.Sign()
			if _, ok := graph.pending[calleeSign]; !ok {
				continue
			}
			if _, seen := seenDeps[calleeSign]; seen {
				continue
			}

			seenDeps[calleeSign] = struct{}{}
			graph.deps[sign] = append(graph.deps[sign], calleeKey)
			graph.reverse[calleeSign] = append(graph.reverse[calleeSign], key)
			graph.inDegree[sign]++
		}
	}

	return graph
}

func (g *depGraph) ready() []*fnKey {
	if g == nil {
		return nil
	}

	ready := make([]*fnKey, 0)
	for sign, key := range g.pending {
		if g.inDegree[sign] == 0 {
			ready = append(ready, key)
		}
	}

	sort.Slice(ready, func(i int, j int) bool {
		return ready[i].Sign() < ready[j].Sign()
	})
	return ready
}

func (g *depGraph) resolve(sign FuncSign) {
	if g == nil {
		return
	}
	if _, ok := g.pending[sign]; !ok {
		return
	}

	delete(g.pending, sign)
	for _, dependent := range g.reverse[sign] {
		if dependent == nil {
			continue
		}

		dependentSign := dependent.Sign()
		if _, ok := g.pending[dependentSign]; !ok {
			continue
		}
		if g.inDegree[dependentSign] > 0 {
			g.inDegree[dependentSign]--
		}
	}
}

func (g *depGraph) remaining() []*fnKey {
	if g == nil {
		return nil
	}

	remaining := make([]*fnKey, 0, len(g.pending))
	for _, key := range g.pending {
		remaining = append(remaining, key)
	}

	sort.Slice(remaining, func(i int, j int) bool {
		return remaining[i].Sign() < remaining[j].Sign()
	})
	return remaining
}

type batch struct {
	keys []*fnKey
}

type runContext struct {
	idx       store.FileIndex
	graph     *depGraph
	summaries map[FuncSign]string
	failed    map[FuncSign]error
}

func newRunContext(idx store.FileIndex) *runContext {
	return &runContext{
		idx:       idx,
		graph:     newDepGraph(idx),
		summaries: seedExistingSummaries(idx),
		failed:    make(map[FuncSign]error),
	}
}

func NewFunctionSummaryAgent(client llm.Client, cfg *FunctionSummaryConfig) *FunctionSummaryAgent {
	normalizedCfg := cloneFunctionSummaryConfig(cfg)
	logger := logpkg.New(false)
	wrappedClient := client
	if wrappedClient != nil && normalizedCfg.MaxRetries > 0 {
		wrappedClient = llm.NewRetryingClient(wrappedClient, normalizedCfg.MaxRetries, logger)
	}

	return &FunctionSummaryAgent{
		client: wrappedClient,
		cfg:    normalizedCfg,
		logger: logger,
	}
}

func cloneFunctionSummaryConfig(cfg *FunctionSummaryConfig) *FunctionSummaryConfig {
	if cfg == nil {
		return &FunctionSummaryConfig{}
	}

	cloned := *cfg
	return &cloned
}

func seedExistingSummaries(idx store.FileIndex) map[FuncSign]string {
	summaries := make(map[FuncSign]string)
	for _, entry := range idx {
		if entry == nil {
			continue
		}
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			summary := strings.TrimSpace(fn.Summary)
			if summary == "" {
				continue
			}
			sign := newFnKey(fn).Sign()
			if sign == "" {
				continue
			}
			summaries[sign] = summary
		}
	}
	return summaries
}

func mergeFunctionSummaryFailures(dst map[FuncSign]error, src map[FuncSign]error) {
	if dst == nil {
		return
	}
	for sign, err := range src {
		if sign == "" || err == nil {
			continue
		}
		if _, exists := dst[sign]; exists {
			continue
		}
		dst[sign] = err
	}
}

func collectBlockedSigns(remaining []*fnKey, failed map[FuncSign]error) []FuncSign {
	blocked := make([]FuncSign, 0, len(remaining))
	seen := make(map[FuncSign]struct{}, len(remaining))
	for _, key := range remaining {
		if key == nil {
			continue
		}
		sign := key.Sign()
		if sign == "" {
			continue
		}
		if _, isFailed := failed[sign]; isFailed {
			continue
		}
		if _, ok := seen[sign]; ok {
			continue
		}
		seen[sign] = struct{}{}
		blocked = append(blocked, sign)
	}
	sort.Slice(blocked, func(i int, j int) bool {
		return blocked[i] < blocked[j]
	})
	return blocked
}

func buildRequestedBatchMap(currentBatch *batch) (map[functionSummaryBatchKey]*fnKey, map[FuncSign]error) {
	requested := make(map[functionSummaryBatchKey]*fnKey)
	failures := make(map[FuncSign]error)
	if currentBatch == nil {
		return requested, failures
	}

	owners := make(map[functionSummaryBatchKey][]*fnKey, len(currentBatch.keys))
	for _, key := range currentBatch.keys {
		if key == nil {
			continue
		}
		requestKey := newFunctionSummaryBatchKey(key)
		owners[requestKey] = append(owners[requestKey], key)
	}

	requestKeys := make([]functionSummaryBatchKey, 0, len(owners))
	for requestKey := range owners {
		requestKeys = append(requestKeys, requestKey)
	}
	sort.Slice(requestKeys, func(i int, j int) bool {
		if requestKeys[i].path == requestKeys[j].path {
			return requestKeys[i].id < requestKeys[j].id
		}
		return requestKeys[i].path < requestKeys[j].path
	})

	for _, requestKey := range requestKeys {
		keys := owners[requestKey]
		if len(keys) == 1 {
			requested[requestKey] = keys[0]
			continue
		}

		signs := make([]string, 0, len(keys))
		for _, key := range keys {
			sign := key.Sign()
			if sign == "" {
				continue
			}
			signs = append(signs, string(sign))
		}
		sort.Strings(signs)

		err := fmt.Errorf("duplicate requested batch item for path %q id %q: [%s]", requestKey.path, requestKey.id, strings.Join(signs, ", "))
		for _, key := range keys {
			sign := key.Sign()
			if sign == "" {
				continue
			}
			failures[sign] = err
		}
	}

	return requested, failures
}

func requestedFailureMap(requested map[functionSummaryBatchKey]*fnKey, err error) map[FuncSign]error {
	failures := make(map[FuncSign]error, len(requested))
	for _, key := range requested {
		sign := key.Sign()
		if sign == "" {
			continue
		}
		failures[sign] = err
	}
	return failures
}

func functionMatchesKey(fn *store.FunctionDecl, key *fnKey) bool {
	if fn == nil || key == nil {
		return false
	}
	if fn.Path != key.path || fn.Name != key.name {
		return false
	}
	if key.receiver == "" {
		return fn.Receiver == ""
	}
	return fn.Receiver == key.receiver
}

func lookupFunctionInEntry(entry *store.FileEntry, key *fnKey) *store.FunctionDecl {
	if entry == nil || key == nil {
		return nil
	}
	for _, fn := range entry.Functions {
		if functionMatchesKey(fn, key) {
			return fn
		}
	}
	return nil
}

func internalCallTarget(call *store.CallRef) (string, string) {
	if call == nil {
		return "", ""
	}

	if target := strings.TrimSpace(call.ResolvedTarget); target != "" {
		path, name, ok := strings.Cut(target, "#")
		if !ok {
			return "", ""
		}
		path = strings.TrimSpace(path)
		name = strings.TrimSpace(name)
		if path == "" || name == "" {
			return "", ""
		}
		return path, name
	}

	path := strings.TrimSpace(call.Path)
	name := strings.TrimSpace(call.Name)
	if path == "" || name == "" {
		return "", ""
	}
	return path, name
}

func lookupInternalCallInEntry(entry *store.FileEntry, call *store.CallRef) *store.FunctionDecl {
	if entry == nil || call == nil {
		return nil
	}

	targetPath, targetName := internalCallTarget(call)
	if targetPath == "" || targetName == "" {
		return nil
	}
	if strings.TrimSpace(entry.Path) != "" && entry.Path != targetPath {
		return nil
	}

	freeFunctions := make([]*store.FunctionDecl, 0, 1)
	methods := make([]*store.FunctionDecl, 0, 1)
	for _, fn := range entry.Functions {
		if fn == nil {
			continue
		}
		if fn.Path != targetPath || fn.Name != targetName {
			continue
		}
		if strings.TrimSpace(fn.Receiver) == "" {
			freeFunctions = append(freeFunctions, fn)
			continue
		}
		methods = append(methods, fn)
	}

	if strings.TrimSpace(call.Receiver) == "" {
		if len(freeFunctions) == 1 {
			return freeFunctions[0]
		}
		return nil
	}

	if len(methods) == 1 && len(freeFunctions) == 0 {
		return methods[0]
	}
	if len(methods) == 0 && len(freeFunctions) == 1 {
		return freeFunctions[0]
	}

	return nil
}

func sortedFileIndexPaths(idx store.FileIndex) []string {
	paths := make([]string, 0, len(idx))
	for path := range idx {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func resolvedInternalCallKey(idx store.FileIndex, call *store.CallRef) *fnKey {
	if call == nil || call.Ownership != store.OwnershipInternal {
		return nil
	}

	targetPath, _ := internalCallTarget(call)
	if targetPath == "" {
		return nil
	}

	if entry := idx[targetPath]; entry != nil {
		if fn := lookupInternalCallInEntry(entry, call); fn != nil {
			return newFnKey(fn)
		}
	}

	return nil
}

func estimateTokens(charCount int) int {
	if charCount <= 0 {
		return 0
	}
	return (charCount + 3) / 4
}

func estimateFunctionTokens(idx store.FileIndex, fn *store.FunctionDecl, summaries map[FuncSign]string) int {
	if fn == nil {
		return 0
	}

	cost := estimateTokens(len(fn.Src)) + 50
	seenSummaries := make(map[FuncSign]struct{})
	for _, call := range fn.Calls {
		calleeKey := resolvedInternalCallKey(idx, call)
		calleeSign := calleeKey.Sign()
		if calleeSign == "" {
			continue
		}
		if _, seen := seenSummaries[calleeSign]; seen {
			continue
		}
		seenSummaries[calleeSign] = struct{}{}

		summary := strings.TrimSpace(summaries[calleeSign])
		if summary == "" {
			continue
		}
		cost += estimateTokens(len(summary))
	}
	return cost
}

func lookupFunction(idx store.FileIndex, key *fnKey) *store.FunctionDecl {
	if key == nil {
		return nil
	}

	if entry := idx[key.path]; entry != nil {
		if fn := lookupFunctionInEntry(entry, key); fn != nil {
			return fn
		}
	}

	for _, path := range sortedFileIndexPaths(idx) {
		if path == key.path {
			continue
		}
		if fn := lookupFunctionInEntry(idx[path], key); fn != nil {
			return fn
		}
	}

	return nil
}

func buildBatches(ready []*fnKey, idx store.FileIndex, contextBudget int, summaries map[FuncSign]string) []*batch {
	batches := make([]*batch, 0)
	current := &batch{keys: make([]*fnKey, 0)}
	currentCost := 0
	flush := func() {
		if len(current.keys) == 0 {
			return
		}
		keys := append([]*fnKey(nil), current.keys...)
		batches = append(batches, &batch{keys: keys})
		current = &batch{keys: make([]*fnKey, 0)}
		currentCost = 0
	}

	for _, key := range ready {
		fn := lookupFunction(idx, key)
		if fn == nil {
			continue
		}
		if strings.TrimSpace(fn.Src) == "" {
			continue
		}

		cost := estimateFunctionTokens(idx, fn, summaries)
		if len(current.keys) > 0 && currentCost+cost > contextBudget {
			flush()
		}
		if len(current.keys) == 0 && cost > contextBudget {
			batches = append(batches, &batch{keys: []*fnKey{key}})
			continue
		}

		current.keys = append(current.keys, key)
		currentCost += cost
	}

	flush()
	return batches
}

func (a *FunctionSummaryAgent) buildPrompt(state *runContext, currentBatch *batch) (*llm.CompletionRequest, error) {
	if a == nil {
		return nil, fmt.Errorf("function summary agent is nil")
	}

	cfg := a.cfg
	if cfg == nil {
		cfg = &FunctionSummaryConfig{}
	}

	data := &promptpkg.FunctionUserPromptData{
		Functions: buildFunctionPromptFunctions(state, currentBatch),
	}

	var userBuf bytes.Buffer
	if err := promptpkg.FunctionUserPromptTmp.Execute(&userBuf, data); err != nil {
		return nil, fmt.Errorf("execute function user prompt: %w", err)
	}

	var systemBuf bytes.Buffer
	if err := promptpkg.FunctionSystemPromptTmp.Execute(&systemBuf, &promptpkg.FunctionSystemPromptData{
		Level:    a.cfg.DependencyDepth - 1,
		Depth:    a.cfg.DependencyDepth,
		Language: a.cfg.Language,
	}); err != nil {
		return nil, fmt.Errorf("execute function system prompt: %w", err)
	}

	return &llm.CompletionRequest{
		Model:     cfg.Model,
		SystemMsg: systemBuf.String(),
		UserMsg:   userBuf.String(),
		MaxTokens: cfg.MaxTokens,
	}, nil
}

func buildFunctionPromptFunctions(state *runContext, currentBatch *batch) []promptpkg.FunctionStruct {
	if state == nil || currentBatch == nil {
		return nil
	}

	functions := make([]promptpkg.FunctionStruct, 0, len(currentBatch.keys))
	for _, key := range currentBatch.keys {
		fn := lookupFunction(state.idx, key)
		if fn == nil {
			continue
		}

		var calledFunctions []*promptpkg.CalledFunctionStruct
		seenCalledFunctions := make(map[FuncSign]struct{})
		for _, call := range fn.Calls {
			calleeKey := resolvedInternalCallKey(state.idx, call)
			sign := calleeKey.Sign()
			if sign == "" {
				continue
			}
			if _, seen := seenCalledFunctions[sign]; seen {
				continue
			}
			seenCalledFunctions[sign] = struct{}{}

			summary := strings.TrimSpace(state.summaries[sign])
			if summary == "" {
				continue
			}
			calledFunctions = append(calledFunctions, &promptpkg.CalledFunctionStruct{
				Name:    string(sign),
				Summary: summary,
			})
		}

		functions = append(functions, promptpkg.FunctionStruct{
			Path:            fn.Path,
			Src:             fn.Src,
			CalledFunctions: calledFunctions,
		})
	}
	return functions
}

func (a *FunctionSummaryAgent) parseResponse(content string) ([]*functionSummaryResult, error) {
	var resp functionSummaryResponse
	if err := llm.ParseJSON(content, &resp); err != nil {
		return nil, err
	}
	return resp.Functions, nil
}

func (a *FunctionSummaryAgent) reconcileBatchResults(requested map[functionSummaryBatchKey]*fnKey, results []*functionSummaryResult) (map[FuncSign]*functionSummaryMatch, map[FuncSign]error) {
	matches := make(map[FuncSign]*functionSummaryMatch)
	failures := make(map[FuncSign]error)

	for _, result := range results {
		if result == nil {
			continue
		}

		resultKey := newFunctionSummaryKey(result.Path, result.ID)
		requestedKey, ok := requested[resultKey]
		if !ok {
			if a != nil && a.logger != nil {
				a.logger.Warn("function summary response item not requested in batch", "path", result.Path, "id", result.ID)
			}
			continue
		}

		sign := requestedKey.Sign()
		if sign == "" {
			continue
		}
		if _, alreadyFailed := failures[sign]; alreadyFailed {
			continue
		}
		if _, seen := matches[sign]; seen {
			if a != nil && a.logger != nil {
				a.logger.Warn("duplicate function summary response item", "path", result.Path, "id", result.ID, "sign", sign)
			}
			failures[sign] = fmt.Errorf("duplicate response item for path %q id %q", resultKey.path, resultKey.id)
			delete(matches, sign)
			continue
		}

		matches[sign] = &functionSummaryMatch{key: requestedKey, result: result}
	}

	requestKeys := make([]functionSummaryBatchKey, 0, len(requested))
	for requestKey := range requested {
		requestKeys = append(requestKeys, requestKey)
	}
	sort.Slice(requestKeys, func(i int, j int) bool {
		if requestKeys[i].path == requestKeys[j].path {
			return requestKeys[i].id < requestKeys[j].id
		}
		return requestKeys[i].path < requestKeys[j].path
	})

	for _, requestKey := range requestKeys {
		requestedKey := requested[requestKey]
		sign := requestedKey.Sign()
		if sign == "" {
			continue
		}
		if _, failed := failures[sign]; failed {
			continue
		}
		if _, ok := matches[sign]; ok {
			continue
		}
		failures[sign] = fmt.Errorf("missing response item for path %q id %q", requestKey.path, requestKey.id)
	}

	return matches, failures
}

func (a *FunctionSummaryAgent) applySummaries(rc *runContext, matches map[FuncSign]*functionSummaryMatch) ([]FuncSign, map[FuncSign]error) {
	if rc == nil {
		return nil, nil
	}
	if rc.summaries == nil {
		rc.summaries = make(map[FuncSign]string)
	}

	signs := make([]FuncSign, 0, len(matches))
	for sign := range matches {
		signs = append(signs, sign)
	}
	sort.Slice(signs, func(i int, j int) bool {
		return signs[i] < signs[j]
	})

	applied := make([]FuncSign, 0, len(signs))
	failures := make(map[FuncSign]error)
	for _, sign := range signs {
		match := matches[sign]
		if match == nil || match.key == nil || match.result == nil {
			if a != nil && a.logger != nil {
				a.logger.Warn("function summary match missing requested key or result", "sign", sign)
			}
			failures[sign] = fmt.Errorf("matched function summary missing requested key or result for %s", sign)
			continue
		}

		fn := lookupFunction(rc.idx, match.key)
		if fn == nil {
			if a != nil && a.logger != nil {
				a.logger.Warn("function summary requested key missing from index", "sign", sign, "path", match.result.Path, "id", match.result.ID)
			}
			failures[sign] = fmt.Errorf("matched function summary missing exact index entry for %s", sign)
			continue
		}

		rc.summaries[sign] = match.result.Summary
		fn.Summary = match.result.Summary
		applied = append(applied, sign)
	}

	return applied, failures
}

func (r *runContext) hasPendingFunctions() bool {
	if r == nil || r.graph == nil {
		return false
	}
	return len(r.graph.pending) > 0
}

func (a *FunctionSummaryAgent) validateRuntime() error {
	if a == nil {
		return fmt.Errorf("function summary agent is nil")
	}

	cfg := a.cfg
	if cfg == nil {
		cfg = &FunctionSummaryConfig{}
	}

	missing := make([]string, 0, 4)
	if a.client == nil {
		missing = append(missing, "client")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		missing = append(missing, "model")
	}
	if cfg.MaxTokens <= 0 {
		missing = append(missing, "max_tokens")
	}
	if cfg.ContextBudget <= 0 {
		missing = append(missing, "context_budget")
	}
	if len(missing) > 0 {
		return fmt.Errorf("function summary agent unusable: missing %s", strings.Join(missing, ", "))
	}

	return nil
}

func (a *FunctionSummaryAgent) processBatch(ctx context.Context, rc *runContext, currentBatch *batch) (map[FuncSign]error, error) {
	requested, failures := buildRequestedBatchMap(currentBatch)
	if len(failures) > 0 {
		return failures, nil
	}
	if len(requested) == 0 {
		return nil, nil
	}

	req, err := a.buildPrompt(rc, currentBatch)
	if err != nil {
		return requestedFailureMap(requested, fmt.Errorf("build function summary prompt: %w", err)), nil
	}

	response, err := a.client.Complete(ctx, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
			return nil, ctxErr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return requestedFailureMap(requested, fmt.Errorf("complete function summary batch: %w", err)), nil
	}

	results, err := a.parseResponse(response)
	if err != nil {
		return requestedFailureMap(requested, fmt.Errorf("parse function summary response: %w", err)), nil
	}

	matches, matchFailures := a.reconcileBatchResults(requested, results)
	applied, applyFailures := a.applySummaries(rc, matches)
	mergeFunctionSummaryFailures(matchFailures, applyFailures)

	for _, sign := range applied {
		if _, failed := matchFailures[sign]; failed {
			continue
		}
		rc.graph.resolve(sign)
	}

	return matchFailures, nil
}

func (a *FunctionSummaryAgent) processLayer(ctx context.Context, rc *runContext, ready []*fnKey) error {
	if len(ready) == 0 {
		return nil
	}

	batches := buildBatches(ready, rc.idx, a.cfg.ContextBudget, rc.summaries)
	layerFailed := make(map[FuncSign]error)
	for _, currentBatch := range batches {
		batchFailed, err := a.processBatch(ctx, rc, currentBatch)
		if err != nil {
			return err
		}
		mergeFunctionSummaryFailures(rc.failed, batchFailed)
		mergeFunctionSummaryFailures(layerFailed, batchFailed)
	}

	if len(layerFailed) > 0 {
		return newFunctionSummaryRunError(layerFailed, collectBlockedSigns(rc.graph.remaining(), layerFailed))
	}

	return nil
}

func (a *FunctionSummaryAgent) Run(ctx context.Context, idx store.FileIndex, metrics store.MetricsMap) error {
	if len(idx) == 0 {
		return nil
	}

	// Filter functions below importance threshold if metrics are available
	if len(metrics) > 0 && a.cfg != nil && a.cfg.ImportanceThreshold > 0 {
		filtered := filterFileIndexByImportance(idx, metrics, a.cfg.ImportanceThreshold)
		total := countFunctions(idx)
		kept := countFunctions(filtered)
		if skipped := total - kept; skipped > 0 {
			a.logger.Debug("filtered functions by importance", "total", total, "kept", kept, "skipped", skipped)
		}
		idx = filtered
	}

	state := newRunContext(idx)
	if !state.hasPendingFunctions() {
		return nil
	}

	if err := a.validateRuntime(); err != nil {
		return err
	}

	for {
		ready := state.graph.ready()
		if len(ready) == 0 {
			break
		}
		if err := a.processLayer(ctx, state, ready); err != nil {
			return err
		}
	}

	remaining := state.graph.remaining()
	if len(remaining) == 0 {
		return nil
	}

	cycleBatch := &batch{keys: remaining}
	if _, failures := buildRequestedBatchMap(cycleBatch); len(failures) > 0 {
		mergeFunctionSummaryFailures(state.failed, failures)
		return newFunctionSummaryRunError(state.failed, collectBlockedSigns(state.graph.remaining(), state.failed))
	}

	batchFailed, err := a.processBatch(ctx, state, cycleBatch)
	if err != nil {
		return err
	}
	mergeFunctionSummaryFailures(state.failed, batchFailed)

	remaining = state.graph.remaining()
	if len(remaining) > 0 || len(state.failed) > 0 {
		return newFunctionSummaryRunError(state.failed, collectBlockedSigns(remaining, state.failed))
	}

	return nil
}

// filterFileIndexByImportance creates a filtered copy of the FileIndex containing only
// functions that meet the importance threshold. FunctionDecl pointers are intentionally
// shared (not copied) with the original index so that summary mutations (fn.Summary = ...)
// propagate back to the caller's index.
func filterFileIndexByImportance(idx store.FileIndex, metrics store.MetricsMap, threshold float64) store.FileIndex {
	filtered := make(store.FileIndex, len(idx))
	for path, entry := range idx {
		if entry == nil {
			continue
		}
		newEntry := &store.FileEntry{
			Language:    entry.Language,
			ContentHash: entry.ContentHash,
			Functions:   make([]*store.FunctionDecl, 0, len(entry.Functions)),
			Types:       entry.Types,
			Imports:     entry.Imports,
			Path:        entry.Path,
		}
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			id := store.FuncID(fn)
			// Include if no metrics (conservative) or score meets threshold
			if m, ok := metrics[id]; !ok || m.ImportanceScore >= threshold {
				newEntry.Functions = append(newEntry.Functions, fn)
			}
		}
		filtered[path] = newEntry
	}
	return filtered
}

func countFunctions(idx store.FileIndex) int {
	count := 0
	for _, entry := range idx {
		if entry == nil {
			continue
		}
		count += len(entry.Functions)
	}
	return count
}
