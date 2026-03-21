package planner

import (
	"fmt"
	"sort"
	"strings"

	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

var logger logpkg.Logger = logpkg.New(false)

func estimateTokens(text string) int {
	return len(text) / 4
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
	droppedSymbols := 0
	for _, line := range unexportedLines {
		candidate := strings.Join(append(selectedLines, line), "\n")
		if estimateTokens(candidate) > maxTokens {
			droppedSymbols++
			continue
		}
		selectedLines = append(selectedLines, line)
	}
	if droppedSymbols > 0 && logger != nil {
		logger.Warn("planner skeleton truncated", "dropped_symbols", droppedSymbols)
	}

	result := strings.Join(selectedLines, "\n")
	if estimateTokens(result) <= maxTokens {
		return result
	}

	trimmedLines := []string{}
	for _, line := range selectedLines {
		candidate := strings.Join(append(trimmedLines, line), "\n")
		if estimateTokens(candidate) > maxTokens {
			break
		}
		trimmedLines = append(trimmedLines, line)
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
