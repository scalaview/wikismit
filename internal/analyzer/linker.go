package analyzer

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/scalaview/wikismit/pkg/store"
)

type functionKey struct {
	PackageDir string
	Name       string
}

type methodKey struct {
	PackageDir   string
	ReceiverType string
	Name         string
}

type linkIndex struct {
	functions map[functionKey][]*store.FunctionDecl
	methods   map[methodKey]*store.FunctionDecl
	fileIndex store.FileIndex
}

type importAliasMap struct {
	aliases map[string]string
	dots    []string
}

type CycleReport struct {
	Cycles [][]string `json:"cycles"`
}

func buildLinkIndex(idx store.FileIndex) *linkIndex {
	li := &linkIndex{
		functions: make(map[functionKey][]*store.FunctionDecl),
		methods:   make(map[methodKey]*store.FunctionDecl),
		fileIndex: idx,
	}

	for _, filePath := range sortedFilePaths(idx) {
		entry := idx[filePath]
		if entry == nil {
			continue
		}
		pkgDir := packageDirForEntry(filePath, entry)
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}
			if fn.FunctionType == store.FunctionTypeMethod && fn.Receiver != "" {
				key := methodKey{
					PackageDir:   pkgDir,
					ReceiverType: normalizeReceiverType(fn.Receiver),
					Name:         fn.Name,
				}
				if existing, ok := li.methods[key]; !ok || functionLess(fn, existing) {
					li.methods[key] = fn
				}
				continue
			}
			key := functionKey{PackageDir: pkgDir, Name: fn.Name}
			li.functions[key] = append(li.functions[key], fn)
		}
	}

	for key := range li.functions {
		sort.Slice(li.functions[key], func(i int, j int) bool {
			return functionLess(li.functions[key][i], li.functions[key][j])
		})
	}

	return li
}

func buildImportAliasMap(entry *store.FileEntry) *importAliasMap {
	am := &importAliasMap{
		aliases: make(map[string]string),
		dots:    make([]string, 0),
	}
	if entry == nil {
		return am
	}

	dotSet := make(map[string]struct{})
	for _, imp := range entry.Imports {
		if imp == nil || !imp.Internal || imp.ResolvedPath == "" {
			continue
		}

		pkgDir := normalizePackageDir(imp.ResolvedPath)
		switch imp.Alias {
		case "_":
			continue
		case ".":
			if _, ok := dotSet[pkgDir]; ok {
				continue
			}
			dotSet[pkgDir] = struct{}{}
			am.dots = append(am.dots, pkgDir)
			continue
		}

		alias := imp.Alias
		if alias == "" {
			alias = path.Base(imp.Path)
		}
		am.aliases[alias] = pkgDir
	}

	sort.Strings(am.dots)
	return am
}

func (li *linkIndex) resolveCrossPackage(call *store.CallRef, am *importAliasMap) bool {
	if li == nil || call == nil || am == nil {
		return false
	}
	pkgDir, ok := am.aliases[call.Receiver]
	if !ok {
		return false
	}
	candidate := li.resolveFunction(pkgDir, call.Name)
	if candidate == nil {
		return false
	}
	setResolvedCall(call, candidate)
	return true
}

func (li *linkIndex) resolveSamePackage(call *store.CallRef, currentPkgDir string) bool {
	if li == nil || call == nil {
		return false
	}
	candidate := li.resolveFunction(currentPkgDir, call.Name)
	if candidate == nil {
		return false
	}
	setResolvedCall(call, candidate)
	return true
}

func (li *linkIndex) resolveDotImport(call *store.CallRef, am *importAliasMap) bool {
	if li == nil || call == nil || am == nil {
		return false
	}
	for _, pkgDir := range am.dots {
		candidate := li.resolveFunction(pkgDir, call.Name)
		if candidate == nil {
			continue
		}
		setResolvedCall(call, candidate)
		return true
	}
	return false
}

func findVarType(fn *store.FunctionDecl, receiverName string, callLine int) string {
	if fn == nil || receiverName == "" {
		return ""
	}

	bestLine := -1
	bestType := ""
	for _, vd := range fn.VarDefs {
		if vd == nil || vd.Name != receiverName || vd.Line >= callLine {
			continue
		}
		if vd.Line > bestLine {
			bestLine = vd.Line
			bestType = vd.Type
		}
	}

	return normalizeReceiverType(bestType)
}

func (li *linkIndex) resolveMethod(call *store.CallRef, fn *store.FunctionDecl, am *importAliasMap, currentPkgDir string) bool {
	if li == nil || call == nil || fn == nil {
		return false
	}

	varType := findVarType(fn, call.Receiver, call.Line)
	if varType == "" {
		return false
	}

	pkgDir := currentPkgDir
	typeName := varType
	if idx := strings.LastIndex(varType, "."); idx >= 0 {
		pkgAlias := varType[:idx]
		typeName = varType[idx+1:]
		resolvedPkgDir, ok := am.aliases[pkgAlias]
		if !ok {
			return false
		}
		pkgDir = resolvedPkgDir
	}

	candidate, ok := li.methods[methodKey{
		PackageDir:   pkgDir,
		ReceiverType: normalizeReceiverType(typeName),
		Name:         call.Name,
	}]
	if !ok {
		return false
	}

	setResolvedCall(call, candidate)
	return true
}

func (li *linkIndex) resolveFunction(pkgDir string, name string) *store.FunctionDecl {
	candidates := li.functions[functionKey{PackageDir: pkgDir, Name: name}]
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func LinkCalls(idx store.FileIndex) store.CallGraph {
	li := buildLinkIndex(idx)
	graph := make(store.CallGraph)

	for _, filePath := range sortedFilePaths(idx) {
		entry := idx[filePath]
		if entry == nil {
			continue
		}

		am := buildImportAliasMap(entry)
		currentPkgDir := packageDirForEntry(filePath, entry)
		for _, fn := range entry.Functions {
			if fn == nil {
				continue
			}

			fnKey := functionTarget(fn)
			edgeSet := make(map[string]struct{})
			for _, call := range fn.Calls {
				if call == nil {
					continue
				}

				call.ResolvedTarget = ""
				call.Ownership = store.OwnershipExternal

				resolved := false
				if call.Receiver != "" {
					resolved = li.resolveCrossPackage(call, am)
				}
				if !resolved && call.Receiver != "" {
					resolved = li.resolveMethod(call, fn, am, currentPkgDir)
				}
				if !resolved && call.Receiver == "" {
					resolved = li.resolveSamePackage(call, currentPkgDir)
					if !resolved {
						resolved = li.resolveDotImport(call, am)
					}
				}
				if call.ResolvedTarget != "" {
					edgeSet[call.ResolvedTarget] = struct{}{}
				}
			}

			if len(edgeSet) == 0 {
				continue
			}

			edges := make([]string, 0, len(edgeSet))
			for edge := range edgeSet {
				edges = append(edges, edge)
			}
			sort.Strings(edges)
			graph[fnKey] = edges
		}
	}

	return graph
}

func DetectCycles(graph store.CallGraph) *CycleReport {
	report := &CycleReport{Cycles: make([][]string, 0)}
	if len(graph) == 0 {
		return report
	}

	const (
		white = iota
		gray
		black
	)

	color := make(map[string]int)
	stack := make([]string, 0, len(graph))
	stackIndex := make(map[string]int)
	seenCycles := make(map[string]struct{})

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		stackIndex[node] = len(stack)
		stack = append(stack, node)

		neighbors := append([]string(nil), graph[node]...)
		sort.Strings(neighbors)
		for _, neighbor := range neighbors {
			switch color[neighbor] {
			case white:
				dfs(neighbor)
			case gray:
				start := stackIndex[neighbor]
				cycle := append([]string(nil), stack[start:]...)
				cycle = append(cycle, neighbor)
				cycle = canonicalizeCycle(cycle)
				key := strings.Join(cycle, "->")
				if _, ok := seenCycles[key]; ok {
					continue
				}
				seenCycles[key] = struct{}{}
				report.Cycles = append(report.Cycles, cycle)
			}
		}

		stack = stack[:len(stack)-1]
		delete(stackIndex, node)
		color[node] = black
	}

	for _, node := range sortedGraphNodes(graph) {
		if color[node] == white {
			dfs(node)
		}
	}

	sort.Slice(report.Cycles, func(i int, j int) bool {
		return cycleLess(report.Cycles[i], report.Cycles[j])
	})

	return report
}

func sortedFilePaths(idx store.FileIndex) []string {
	paths := make([]string, 0, len(idx))
	for filePath := range idx {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	return paths
}

func packageDirForEntry(filePath string, entry *store.FileEntry) string {
	canonicalPath := canonicalFilePath(filePath, entry)
	return normalizePackageDir(canonicalPath)
}

func canonicalFilePath(filePath string, entry *store.FileEntry) string {
	normalizedKey := filepath.ToSlash(filePath)
	if entry == nil {
		return normalizedKey
	}

	normalizedEntryPath := filepath.ToSlash(entry.Path)
	if normalizedEntryPath == "" {
		return normalizedKey
	}
	if normalizedKey == "" {
		return normalizedEntryPath
	}
	if normalizedEntryPath == normalizedKey || strings.HasSuffix(normalizedEntryPath, "/"+normalizedKey) {
		return normalizedKey
	}
	return normalizedEntryPath
}

func normalizePackageDir(filePath string) string {
	return filepath.ToSlash(filepath.Dir(filePath))
}

func normalizeReceiverType(receiver string) string {
	return strings.TrimPrefix(receiver, "*")
}

func functionTarget(fn *store.FunctionDecl) string {
	if fn == nil {
		return ""
	}
	return fn.Path + "#" + fn.Name
}

func setResolvedCall(call *store.CallRef, fn *store.FunctionDecl) {
	call.ResolvedTarget = functionTarget(fn)
	call.Path = fn.Path
	call.Ownership = store.OwnershipInternal
}

func functionLess(left *store.FunctionDecl, right *store.FunctionDecl) bool {
	if left == nil {
		return right != nil
	}
	if right == nil {
		return false
	}
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	if left.LineStart != right.LineStart {
		return left.LineStart < right.LineStart
	}
	if left.Name != right.Name {
		return left.Name < right.Name
	}
	return left.Receiver < right.Receiver
}

func sortedGraphNodes(graph store.CallGraph) []string {
	nodeSet := make(map[string]struct{})
	for node, edges := range graph {
		nodeSet[node] = struct{}{}
		for _, edge := range edges {
			nodeSet[edge] = struct{}{}
		}
	}

	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	return nodes
}

func canonicalizeCycle(cycle []string) []string {
	if len(cycle) <= 1 {
		return append([]string(nil), cycle...)
	}

	body := append([]string(nil), cycle[:len(cycle)-1]...)
	best := rotateCycle(body, 0)
	for idx := 1; idx < len(body); idx++ {
		candidate := rotateCycle(body, idx)
		if compareStringSlices(candidate, best) < 0 {
			best = candidate
		}
	}

	best = append(best, best[0])
	return best
}

func rotateCycle(nodes []string, start int) []string {
	rotated := make([]string, 0, len(nodes))
	rotated = append(rotated, nodes[start:]...)
	rotated = append(rotated, nodes[:start]...)
	return rotated
}

func compareStringSlices(left []string, right []string) int {
	for idx := 0; idx < len(left) && idx < len(right); idx++ {
		if left[idx] < right[idx] {
			return -1
		}
		if left[idx] > right[idx] {
			return 1
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

func cycleLess(left []string, right []string) bool {
	return compareStringSlices(left, right) < 0
}
