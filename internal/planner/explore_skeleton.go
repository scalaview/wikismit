package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

// SkeletonFilterConfig controls which functions are included in the exploration skeleton.
type SkeletonFilterConfig struct {
	MinFuncLines      int     // minimum lines of code (default: 5)
	MinCalledBy       int     // minimum CalledBy count (default: 1)
	MinImportance     float64 // minimum ImportanceScore (default: 0.05)
	IncludeEventHints bool    // include likely event hints in Round 1 skeleton
}

// DefaultSkeletonFilterConfig returns the recommended filter configuration.
func DefaultSkeletonFilterConfig() SkeletonFilterConfig {
	return SkeletonFilterConfig{
		MinFuncLines:      5,
		MinCalledBy:       1,
		MinImportance:     0.05,
		IncludeEventHints: false,
	}
}

// shouldIncludeFunction decides whether a function appears in the exploration skeleton.
func shouldIncludeFunction(fn *store.FunctionDecl, m *store.FunctionMetrics, cbc int, cfg SkeletonFilterConfig) bool {
	if m != nil && m.IsEntryPoint {
		return true
	}
	if isWiringLikeFunction(fn, cbc, cfg.IncludeEventHints) {
		return true
	}
	if fn.Exported && cbc > 0 {
		return true
	}
	if cbc >= 3 {
		return true
	}
	if m != nil {
		if m.LinesOfCode < cfg.MinFuncLines {
			return false
		}
		if m.ImportanceScore < cfg.MinImportance {
			return false
		}
	}
	if cbc < cfg.MinCalledBy {
		return false
	}
	return true
}

func isWiringLikeFunction(fn *store.FunctionDecl, cbc int, includeHints bool) bool {
	if fn == nil {
		return false
	}
	name := strings.ToLower(fn.Name)
	for _, needle := range []string{"registerhandlers", "setuprouter", "initbus", "bootstrapevents"} {
		if strings.Contains(name, needle) {
			return true
		}
	}
	path := strings.ToLower(fn.Path)
	for _, needle := range []string{"bootstrap/", "router/", "setup/", "wiring/", "register/"} {
		if strings.Contains(path, needle) {
			return fn.Exported || cbc > 0 || hasVisibleEventLandmarks(fn, includeHints)
		}
	}
	return false
}

// countOutDegree returns the number of outbound calls for a function.
func countOutDegree(fn *store.FunctionDecl) int {
	if fn == nil {
		return 0
	}
	return len(fn.Calls)
}

// hasEntryPoint checks if any function in the file is an entry point.
func hasEntryPoint(entry *store.FileEntry, filter *metrics.ImportanceFilter) bool {
	if filter == nil {
		return false
	}
	for _, fn := range entry.Functions {
		id := store.FuncID(fn)
		if m, ok := filter.Metrics()[id]; ok && m.IsEntryPoint {
			return true
		}
	}
	return false
}

// BuildExploreSkeleton produces the enriched skeleton for project structure exploration.
// If filter is nil, falls back to BuildPlannerSkeleton.
func BuildExploreSkeleton(idx store.FileIndex, maxTokens int, filter *metrics.ImportanceFilter, cfg SkeletonFilterConfig) string {
	if filter == nil {
		return BuildPlannerSkeleton(idx, maxTokens)
	}

	sortedFiles := sortFilesByMaxImportance(idx, filter)

	var lines []string
	chars := 0

	for _, file := range sortedFiles {
		entry, ok := idx[file]
		if !ok {
			continue
		}

		fileLines, _ := buildExploreFileBlock(file, entry, filter, cfg)
		if len(fileLines) == 0 {
			continue
		}

		testChars := chars
		wouldExceed := false
		for _, l := range fileLines {
			if estimatedTokensAfterAppend(testChars, l) > maxTokens {
				wouldExceed = true
				break
			}
			if testChars > 0 {
				testChars++
			}
			testChars += len(l)
		}
		if wouldExceed {
			if logger != nil {
				logger.Warn("explore skeleton truncated", "file", file)
			}
			break
		}

		for _, l := range fileLines {
			lines, chars = appendLineWithCharCount(lines, chars, l)
		}
	}

	return strings.Join(lines, "\n")
}

// buildExploreFileBlock builds all lines for a single file in the exploration skeleton.
func buildExploreFileBlock(file string, entry *store.FileEntry, filter *metrics.ImportanceFilter, cfg SkeletonFilterConfig) ([]string, int) {
	var fileLines []string
	fileChars := 0

	var includedFns []*store.FunctionDecl
	totalCalls := 0
	totalCalledBy := 0
	isEntry := hasEntryPoint(entry, filter)

	for _, fn := range entry.Functions {
		id := store.FuncID(fn)
		mm := filter.Metrics()
		var m *store.FunctionMetrics
		if mm != nil {
			m = mm[id]
		}
		cbc := calledByCount(fn)
		if !shouldIncludeFunction(fn, m, cbc, cfg) {
			continue
		}
		includedFns = append(includedFns, fn)
		totalCalls += countOutDegree(fn)
		totalCalledBy += cbc
	}

	if len(includedFns) == 0 {
		return nil, 0
	}

	sort.SliceStable(includedFns, func(i, j int) bool {
		leftHasLandmarks := hasVisibleEventLandmarks(includedFns[i], cfg.IncludeEventHints)
		rightHasLandmarks := hasVisibleEventLandmarks(includedFns[j], cfg.IncludeEventHints)
		if leftHasLandmarks != rightHasLandmarks {
			return leftHasLandmarks
		}
		return false
	})

	header := fmt.Sprintf("// %s", file)
	if isEntry {
		header += " [entry]"
	}
	header += fmt.Sprintf(" calls=%d called_by=%d", totalCalls, totalCalledBy)
	fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, header)

	var importPaths []string
	for _, imp := range entry.Imports {
		if imp.Internal && imp.ResolvedPath != "" {
			importPaths = append(importPaths, imp.ResolvedPath)
		}
	}
	if len(importPaths) > 0 {
		impLine := fmt.Sprintf("  -> %s", strings.Join(importPaths, ", "))
		fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, impLine)
	}

	for _, fn := range includedFns {
		id := store.FuncID(fn)
		marker := "  "
		if filter.IsImportant(id) {
			marker = "  ★ "
		}
		lineCount := fn.LineEnd - fn.LineStart + 1
		sig := formatShortSignature(fn.Signature)
		fnLine := fmt.Sprintf("%s%s//%d", marker, sig, lineCount)
		fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, fnLine)
		for _, landmark := range eventLandmarkLines(fn, cfg.IncludeEventHints) {
			fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, landmark)
		}
	}

	return fileLines, fileChars
}

func eventLandmarkLines(fn *store.FunctionDecl, includeHints bool) []string {
	if fn == nil {
		return nil
	}
	var lines []string
	appendFacts := func(prefix string, facts []*store.EventFact) {
		for _, fact := range facts {
			if fact == nil || strings.TrimSpace(fact.EventName) == "" {
				continue
			}
			line := fmt.Sprintf("    %s %s", prefix, strings.TrimSpace(fact.EventName))
			if fact.HandlerRef != "" {
				line += fmt.Sprintf(" -> %s", fact.HandlerRef)
			}
			lines = append(lines, line)
		}
	}
	if fn.EventFacts != nil {
		appendFacts("event publish:", fn.EventFacts.Publishes)
		appendFacts("event handle:", fn.EventFacts.Handles)
		appendFacts("event register:", fn.EventFacts.Registers)
	}
	if includeHints && fn.EventHints != nil {
		appendFacts("event hint publish:", fn.EventHints.LikelyPublishes)
		appendFacts("event hint handle:", fn.EventHints.LikelyHandles)
		appendFacts("event hint register:", fn.EventHints.LikelyRegisters)
	}
	return lines
}

func hasVisibleEventLandmarks(fn *store.FunctionDecl, includeHints bool) bool {
	return len(eventLandmarkLines(fn, includeHints)) > 0
}
