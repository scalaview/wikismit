package preprocessor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/scalaview/wikismit/pkg/store"
)

func writeSharedModuleMarkdown(artifactsDir string, moduleID string, summary store.SharedSummary) error {
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(moduleDocsDir, moduleID+".md"), []byte(renderSharedModuleMarkdown(moduleID, summary)), 0o644)
}

func renderSharedModuleMarkdown(moduleID string, summary store.SharedSummary) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(moduleID)
	b.WriteString("\n\n")
	b.WriteString(summary.Summary)
	b.WriteString("\n")

	if len(summary.KeyTypes) > 0 {
		b.WriteString("\n## Key Types\n\n")
		for _, keyType := range summary.KeyTypes {
			b.WriteString("- `")
			b.WriteString(keyType)
			b.WriteString("`\n")
		}
	}

	if len(summary.KeyFunctions) > 0 {
		b.WriteString("\n## Key Functions\n\n")
		for _, keyFn := range summary.KeyFunctions {
			b.WriteString("- `")
			b.WriteString(keyFn.Name)
			b.WriteString("`")
			if keyFn.Signature != "" {
				b.WriteString(": `")
				b.WriteString(keyFn.Signature)
				b.WriteString("`")
			}
			if keyFn.Ref != "" {
				b.WriteString(" ([source](")
				b.WriteString(keyFn.Ref)
				b.WriteString("))")
			}
			b.WriteString("\n")
		}
	}

	if len(summary.SourceRefs) > 0 {
		b.WriteString("\n## Source References\n\n")
		for _, ref := range summary.SourceRefs {
			b.WriteString("- `")
			b.WriteString(ref)
			b.WriteString("`\n")
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}
