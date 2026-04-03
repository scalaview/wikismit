package agent

import (
	"github.com/scalaview/wikismit/pkg/store"
)

func RecursionGenFunctionSummary(file string, fnName string, idx store.FileIndex) string {
	entry, ok := idx[file]
	if !ok {
		return ""
	}

	for _, fn := range entry.Functions {
		if fn.Name == fnName && fn.Summary != "" {
			return fn.Summary
		}
	}

	return ""
}
