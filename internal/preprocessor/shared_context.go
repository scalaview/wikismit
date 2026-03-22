package preprocessor

import (
	"sort"
	"strconv"

	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

var sharedLogger logpkg.Logger = logpkg.New(false)

func groundSharedSummaryRefs(summary store.SharedSummary, moduleFiles []string, idx store.FileIndex) store.SharedSummary {
	grounded := summary
	grounded.KeyFunctions = append([]store.KeyFunction(nil), summary.KeyFunctions...)
	sortedFiles := append([]string(nil), moduleFiles...)
	sort.Strings(sortedFiles)

	sourceRefs := make([]string, 0, len(grounded.KeyFunctions))
	for i, keyFn := range grounded.KeyFunctions {
		ref := keyFn.Ref
		for _, file := range sortedFiles {
			entry, ok := idx[file]
			if !ok {
				continue
			}
			for _, fn := range entry.Functions {
				if fn.Name != keyFn.Name {
					continue
				}
				if keyFn.Signature != "" && fn.Signature != keyFn.Signature {
					continue
				}
				ref = file + "#L" + strconv.Itoa(fn.LineStart)
				grounded.KeyFunctions[i].Ref = ref
				goto nextFunction
			}
		}

		grounded.KeyFunctions[i].Ref = ref
		if sharedLogger != nil {
			sharedLogger.Warn("hallucinated ref", "ref", ref)
		}

	nextFunction:
		sourceRefs = append(sourceRefs, grounded.KeyFunctions[i].Ref)
	}

	sort.Strings(sourceRefs)
	grounded.SourceRefs = sourceRefs
	return grounded
}
