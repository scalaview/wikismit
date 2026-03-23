package composer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

var nonAnchorCharRegex = regexp.MustCompile(`[^a-z0-9\- ]+`)

func GenerateTOC(content string) string {
	lines := strings.Split(content, "\n")
	tocLines := make([]string, 0)
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			tocLines = append(tocLines, "- ["+heading+"](#"+anchorForHeading(heading)+")")
			continue
		}
		if strings.HasPrefix(line, "### ") {
			heading := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			tocLines = append(tocLines, "  - ["+heading+"](#"+anchorForHeading(heading)+")")
		}
	}

	if len(tocLines) == 0 {
		return content
	}

	tocBlock := "## Contents\n\n" + strings.Join(tocLines, "\n") + "\n\n"
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			prefix := strings.Join(lines[:i+1], "\n")
			suffix := strings.Join(lines[i+1:], "\n")
			return prefix + "\n\n" + tocBlock + suffix
		}
	}

	return tocBlock + content
}

func anchorForHeading(heading string) string {
	anchor := strings.ToLower(heading)
	anchor = nonAnchorCharRegex.ReplaceAllString(anchor, "")
	anchor = strings.ReplaceAll(anchor, " ", "-")
	anchor = strings.Trim(anchor, "-")
	for strings.Contains(anchor, "--") {
		anchor = strings.ReplaceAll(anchor, "--", "-")
	}
	return anchor
}

func CopyModuleDocs(artifactsDir string, docsDir string, plan *store.NavPlan, symbolMap map[string]string) error {
	for _, module := range plan.Modules {
		sourcePath := filepath.Join(artifactsDir, "module_docs", module.ID+".md")
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			if os.IsNotExist(err) && module.Shared {
				continue
			}
			return err
		}

		rendered := InjectCitations(string(content), symbolMap)
		rendered = GenerateTOC(rendered)

		targetDir := filepath.Join(docsDir, "modules")
		if module.Shared {
			targetDir = filepath.Join(docsDir, "shared")
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(targetDir, module.ID+".md"), []byte(rendered), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func GenerateIndexPage(plan *store.NavPlan, graph store.DepGraph) string {
	modules := append([]store.Module(nil), plan.Modules...)
	sort.Slice(modules, func(i int, j int) bool {
		leftDepth := dependencyDepth(modules[i].ID, graph, map[string]bool{})
		rightDepth := dependencyDepth(modules[j].ID, graph, map[string]bool{})
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return modules[i].ID < modules[j].ID
	})

	var builder strings.Builder
	builder.WriteString("# Documentation Index\n\n")
	builder.WriteString("| Module | Type | Used By |\n")
	builder.WriteString("| --- | --- | --- |\n")
	for _, module := range modules {
		moduleType := "module"
		usedBy := "-"
		if module.Shared {
			moduleType = "shared"
			if len(module.ReferencedBy) > 0 {
				usedBy = strings.Join(module.ReferencedBy, ", ")
			}
		}
		builder.WriteString(fmt.Sprintf("| %s | %s | %s |\n", module.ID, moduleType, usedBy))
	}

	return builder.String()
}

func dependencyDepth(moduleID string, graph store.DepGraph, seen map[string]bool) int {
	if seen[moduleID] {
		return 0
	}
	seen[moduleID] = true

	deps := graph[moduleID]
	maxDepth := 0
	for _, dep := range deps {
		depth := 1 + dependencyDepth(dep, graph, seen)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	delete(seen, moduleID)
	return maxDepth
}

func RunComposer(cfg *configpkg.Config, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) error {
	symbolMap := buildSymbolMap(idx)
	if err := CopyModuleDocs(cfg.ArtifactsDir, cfg.OutputDir, plan, symbolMap); err != nil {
		return err
	}

	indexContent := GenerateIndexPage(plan, graph)
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "index.md"), []byte(indexContent), 0o644); err != nil {
		return err
	}

	report, err := ValidateDocs(cfg.OutputDir)
	if err != nil {
		return err
	}
	if err := store.WriteValidationReport(cfg.ArtifactsDir, report); err != nil {
		return err
	}

	vitepressConfig, err := GenerateVitePressConfig(plan, graph, cfg)
	if err != nil {
		return err
	}
	if err := WriteVitePressAssets(cfg.OutputDir, vitepressConfig, cfg); err != nil {
		return err
	}

	return nil
}
