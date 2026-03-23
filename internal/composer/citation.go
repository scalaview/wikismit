package composer

import (
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/scalaview/wikismit/pkg/store"
)

var citationRegex = regexp.MustCompile("`([A-Z][a-zA-Z0-9]*)`")

func buildSymbolMap(idx store.FileIndex) map[string]string {
	type candidate struct {
		ref      string
		exported bool
		path     string
	}

	symbolCandidates := make(map[string][]candidate)

	for path, entry := range idx {
		for _, fn := range entry.Functions {
			symbolCandidates[fn.Name] = append(symbolCandidates[fn.Name], candidate{
				ref:      path + "#L" + strconv.Itoa(fn.LineStart),
				exported: fn.Exported,
				path:     path,
			})
		}

		for _, typ := range entry.Types {
			symbolCandidates[typ.Name] = append(symbolCandidates[typ.Name], candidate{
				ref:      path + "#L" + strconv.Itoa(typ.LineStart),
				exported: typ.Exported,
				path:     path,
			})
		}
	}

	symbolMap := make(map[string]string)
	for name, candidates := range symbolCandidates {
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate.exported {
				filtered = append(filtered, candidate)
			}
		}
		if len(filtered) == 0 {
			continue
		}

		sort.Slice(filtered, func(i int, j int) bool {
			return filtered[i].path < filtered[j].path
		})
		symbolMap[name] = filtered[0].ref
	}

	return symbolMap
}

func InjectCitations(content string, symbolMap map[string]string) string {
	return citationRegex.ReplaceAllStringFunc(content, func(match string) string {
		symbol := match[1 : len(match)-1]
		if ref, exists := symbolMap[symbol]; exists {
			return "[" + symbol + "](" + ref + ")"
		}
		return match
	})
}

func ProcessFile(path string, symbolMap map[string]string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	updated := InjectCitations(string(content), symbolMap)
	return os.WriteFile(path, []byte(updated), 0o644)
}
