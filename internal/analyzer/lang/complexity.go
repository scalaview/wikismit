package lang

import "strings"

// ComplexityExtractor extracts language-specific complexity metrics.
type ComplexityExtractor interface {
	LinesOfCode(src string) int
}

// LinesOfCodeExtractor is the default language-agnostic implementation.
type LinesOfCodeExtractor struct{}

func (e *LinesOfCodeExtractor) LinesOfCode(src string) int {
	if src == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
