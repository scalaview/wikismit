package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/scalaview/wikismit/internal/metrics"
	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

var logger logpkg.Logger = logpkg.New(false)

func estimateTokens(text string) int {
	return estimateTokensFromChars(len(text))
}

func estimateTokensFromChars(chars int) int {
	return chars / 4
}

func estimateTokensForLines(lines []string) int {
	if len(lines) == 0 {
		return 0
	}

	chars := 0
	for i, line := range lines {
		if i > 0 {
			chars++
		}
		chars += len(line)
	}
	return estimateTokensFromChars(chars)
}

func estimatedTokensAfterAppend(currentChars int, line string) int {
	chars := currentChars + len(line)
	if currentChars > 0 {
		chars++
	}
	return estimateTokensFromChars(chars)
}

func appendLineWithCharCount(lines []string, currentChars int, line string) ([]string, int) {
	if currentChars > 0 {
		currentChars++
	}
	currentChars += len(line)
	return append(lines, line), currentChars
}

func BuildSkeleton(files []string, idx store.FileIndex, maxTokens int) string {
	sortedFiles := append([]string(nil), files...)
	sort.Strings(sortedFiles)

	var exportedLines []string
	var unexportedLines []string
	for _, file := range sortedFiles {
		entry, ok := idx[file]
		if !ok {
			continue
		}

		exportedLines = append(exportedLines, fmt.Sprintf("// === %s ===", file))
		for _, fn := range entry.Functions {
			line := fmt.Sprintf("%s  // %s:%d", fn.Signature, file, fn.LineStart)
			if fn.Exported {
				exportedLines = append(exportedLines, line)
				continue
			}
			unexportedLines = append(unexportedLines, line)
		}
		for _, typ := range entry.Types {
			line := fmt.Sprintf("type %s %s  // %s:%d", typ.Name, typ.Kind, file, typ.LineStart)
			if typ.Exported {
				exportedLines = append(exportedLines, line)
				continue
			}
			unexportedLines = append(unexportedLines, line)
		}
	}

	selectedLines := append([]string(nil), exportedLines...)
	selectedChars := 0
	for i, line := range selectedLines {
		if i > 0 {
			selectedChars++
		}
		selectedChars += len(line)
	}
	droppedSymbols := 0
	for _, line := range unexportedLines {
		if estimatedTokensAfterAppend(selectedChars, line) > maxTokens {
			droppedSymbols++
			continue
		}
		selectedLines, selectedChars = appendLineWithCharCount(selectedLines, selectedChars, line)
	}
	if droppedSymbols > 0 && logger != nil {
		logger.Warn("planner skeleton truncated", "dropped_symbols", droppedSymbols)
	}

	result := strings.Join(selectedLines, "\n")
	if estimateTokensForLines(selectedLines) <= maxTokens {
		return result
	}

	trimmedLines := []string{}
	trimmedChars := 0
	for _, line := range selectedLines {
		if estimatedTokensAfterAppend(trimmedChars, line) > maxTokens {
			logger.Warn("planner skeleton truncated", "dropped_lines", len(selectedLines)-len(trimmedLines), "max_tokens", maxTokens)
			break
		}
		trimmedLines, trimmedChars = appendLineWithCharCount(trimmedLines, trimmedChars, line)
	}
	return strings.Join(trimmedLines, "\n")
}

func BuildSkeletonOnlyWithSummary(files []string, idx store.FileIndex, maxTokens int) string {
	sortedFiles := append([]string(nil), files...)
	sort.Strings(sortedFiles)

	var exportedLines []string
	var unexportedLines []string
	for _, file := range sortedFiles {
		entry, ok := idx[file]
		if !ok {
			continue
		}

		exportedLines = append(exportedLines, fmt.Sprintf("<file: %s>", file))
		for _, fn := range entry.Functions {
			line := fmt.Sprintf("%s  // %d,%d\ndescription:%s", fn.Signature, fn.LineStart, fn.LineEnd, fn.Summary)
			if fn.Exported {
				exportedLines = append(exportedLines, line)
				continue
			}
			unexportedLines = append(unexportedLines, line)
		}

		for _, typ := range entry.Types {
			line := fmt.Sprintf("%s// %d,%d", typ.Src, typ.LineStart, typ.LineEnd)
			if typ.Exported {
				exportedLines = append(exportedLines, line)
				continue
			}
			unexportedLines = append(unexportedLines, line)
		}
		exportedLines = append(exportedLines, "</file>")
	}

	selectedLines := append([]string(nil), exportedLines...)
	selectedChars := 0
	for i, line := range selectedLines {
		if i > 0 {
			selectedChars++
		}
		selectedChars += len(line)
	}
	droppedSymbols := 0
	for _, line := range unexportedLines {
		if estimatedTokensAfterAppend(selectedChars, line) > maxTokens {
			droppedSymbols++
			continue
		}
		selectedLines, selectedChars = appendLineWithCharCount(selectedLines, selectedChars, line)
	}
	if droppedSymbols > 0 && logger != nil {
		logger.Warn("planner skeleton truncated, max tokens reached", "dropped_symbols", droppedSymbols)
	}

	result := strings.Join(selectedLines, "\n")
	if estimateTokensForLines(selectedLines) <= maxTokens {
		return result
	}

	trimmedLines := []string{}
	trimmedChars := 0
	for _, line := range selectedLines {
		if estimatedTokensAfterAppend(trimmedChars, line) > maxTokens {
			logger.Warn("planner skeleton truncated, max tokens reached", "dropped_lines", len(selectedLines)-len(trimmedLines), "max_tokens", maxTokens)
			break
		}
		trimmedLines, trimmedChars = appendLineWithCharCount(trimmedLines, trimmedChars, line)
	}
	return strings.Join(trimmedLines, "\n")
}

func BuildFullSkeleton(idx store.FileIndex, maxTokens int) string {
	files := make([]string, 0, len(idx))
	for file := range idx {
		files = append(files, file)
	}
	return BuildSkeleton(files, idx, maxTokens)
}

// BuildPlannerSkeleton produces a minimal skeleton for the Planner.
// It outputs file paths, exported type names, and internal import relationships.
// Function signatures are excluded, achieving ~70-80% compression vs BuildFullSkeleton.
// Truncation is file-granular: entire files are included or skipped.
func BuildPlannerSkeleton(idx store.FileIndex, maxTokens int) string {
	sortedFiles := make([]string, 0, len(idx))
	for file := range idx {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Strings(sortedFiles)

	var lines []string
	chars := 0

	for _, file := range sortedFiles {
		entry, ok := idx[file]
		if !ok {
			continue
		}

		// Build all output lines for this file first
		var fileLines []string
		fileChars := 0

		// File path header
		header := fmt.Sprintf("// %s", file)
		fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, header)

		// Exported type names (comma-separated on one line)
		var typeNames []string
		for _, typ := range entry.Types {
			if typ.Exported {
				typeNames = append(typeNames, typ.Name)
			}
		}
		if len(typeNames) > 0 {
			typeLine := fmt.Sprintf("  type %s", strings.Join(typeNames, ", "))
			fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, typeLine)
		}

		// Internal import relationships
		var importPaths []string
		for _, imp := range entry.Imports {
			if imp.Internal && imp.ResolvedPath != "" {
				importPaths = append(importPaths, imp.ResolvedPath)
			}
		}
		if len(importPaths) > 0 {
			importLine := fmt.Sprintf("  -> %s", strings.Join(importPaths, ", "))
			fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, importLine)
		}

		// Check token budget at file granularity
		testChars := chars
		wouldExceed := false
		for _, l := range fileLines {
			if estimatedTokensAfterAppend(testChars, l) > maxTokens {
				wouldExceed = true
				break
			}
			testChars += len(l)
			if testChars > 0 {
				testChars++
			}
		}

		if wouldExceed {
			if logger != nil {
				logger.Warn("planner skeleton truncated", "file", file)
			}
			break
		}

		// Include entire file
		for _, l := range fileLines {
			lines, chars = appendLineWithCharCount(lines, chars, l)
		}
	}

	return strings.Join(lines, "\n")
}

// BuildPlannerSkeletonWithImportance builds a planner skeleton with importance markers.
// Files with higher-importance functions appear first. Important functions (above 75th
// percentile) are prefixed with "★ " to guide the planner's attention.
func BuildPlannerSkeletonWithImportance(idx store.FileIndex, maxTokens int, filter *metrics.ImportanceFilter) string {
	if filter == nil {
		return BuildPlannerSkeleton(idx, maxTokens)
	}

	// Sort files: files with important functions come first
	sortedFiles := sortFilesByMaxImportance(idx, filter)

	var lines []string
	chars := 0

	for _, file := range sortedFiles {
		entry, ok := idx[file]
		if !ok {
			continue
		}

		// Build all output lines for this file
		var fileLines []string
		fileChars := 0

		// File path header
		header := fmt.Sprintf("// %s", file)
		fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, header)

		// Exported type names (same as original)
		var typeNames []string
		for _, typ := range entry.Types {
			if typ.Exported {
				typeNames = append(typeNames, typ.Name)
			}
		}
		if len(typeNames) > 0 {
			typeLine := fmt.Sprintf("  type %s", strings.Join(typeNames, ", "))
			fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, typeLine)
		}

		// Internal import relationships (same as original)
		var importPaths []string
		for _, imp := range entry.Imports {
			if imp.Internal && imp.ResolvedPath != "" {
				importPaths = append(importPaths, imp.ResolvedPath)
			}
		}
		if len(importPaths) > 0 {
			importLine := fmt.Sprintf("  -> %s", strings.Join(importPaths, ", "))
			fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, importLine)
		}

		// Functions with importance markers (NEW - not in original BuildPlannerSkeleton)
		for _, fn := range entry.Functions {
			marker := ""
			id := store.FuncID(fn)
			if filter.IsImportant(id) {
				marker = "★ "
			}
			sig := formatShortSignature(fn.Signature)
			fnLine := fmt.Sprintf("  %s%s  // %s:%d", marker, sig, file, fn.LineStart)
			fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, fnLine)
		}

		// Check token budget at file granularity
		testChars := chars
		wouldExceed := false
		for _, l := range fileLines {
			if estimatedTokensAfterAppend(testChars, l) > maxTokens {
				wouldExceed = true
				break
			}
			testChars += len(l)
			if testChars > 0 {
				testChars++
			}
		}

		if wouldExceed {
			if logger != nil {
				logger.Warn("importance skeleton truncated", "file", file)
			}
			break
		}

		for _, l := range fileLines {
			lines, chars = appendLineWithCharCount(lines, chars, l)
		}
	}

	return strings.Join(lines, "\n")
}

// sortFilesByMaxImportance returns file paths sorted by their highest importance score.
func sortFilesByMaxImportance(idx store.FileIndex, filter *metrics.ImportanceFilter) []string {
	fileMaxScore := make(map[string]float64)
	for path, entry := range idx {
		maxScore := 0.0
		for _, fn := range entry.Functions {
			id := store.FuncID(fn)
			if m, ok := filter.Metrics()[id]; ok && m.ImportanceScore > maxScore {
				maxScore = m.ImportanceScore
			}
		}
		fileMaxScore[path] = maxScore
	}

	paths := make([]string, 0, len(idx))
	for p := range idx {
		paths = append(paths, p)
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return fileMaxScore[paths[i]] > fileMaxScore[paths[j]]
	})
	return paths
}

// formatShortSignature truncates long signatures for skeleton display.
func formatShortSignature(sig string) string {
	if len(sig) > 80 {
		return sig[:77] + "..."
	}
	return sig
}

// calledByCount returns the number of callers for a function.
func calledByCount(fn *store.FunctionDecl) int {
	if fn == nil {
		return 0
	}
	return len(fn.CalledBy)
}
