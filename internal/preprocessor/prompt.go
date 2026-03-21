package preprocessor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/scalaview/wikismit/pkg/store"
)

func buildSharedPrompt(moduleID string, skeleton string, alreadySummarised store.SharedContext) string {
	var dependencyBlock string
	if len(alreadySummarised) > 0 {
		keys := make([]string, 0, len(alreadySummarised))
		for key := range alreadySummarised {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		var lines []string
		lines = append(lines, "The following shared modules are used by this module.")
		lines = append(lines, "Use their summaries for context only — do not describe them:")
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("- %s: %s", key, alreadySummarised[key].Summary))
		}
		dependencyBlock = strings.Join(lines, "\n") + "\n\n"
	}

	return fmt.Sprintf(`You are documenting the shared module %q.

Code skeleton:
%s

%sRespond ONLY with valid JSON:
{
  "summary": "2-4 sentence description of purpose and usage pattern",
  "key_types": ["TypeName1", "TypeName2"],
  "key_functions": [
    {"name": "FuncName", "signature": "func ...", "ref": "path/file.go#L18"}
  ]
}
`, moduleID, skeleton, dependencyBlock)
}
