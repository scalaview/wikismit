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
