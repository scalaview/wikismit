package agent

import (
	"fmt"
	"strings"

	"github.com/scalaview/wikismit/internal/planner"
	"github.com/scalaview/wikismit/pkg/store"
)

func BuildAgentPrompt(input AgentInput) string {
	skeleton := planner.BuildSkeleton(input.Module.Files, input.FileIndex, input.Config.Agent.SkeletonMaxTokens)
	sharedBlock := buildSharedModulesBlock(input)

	sections := []string{
		fmt.Sprintf("You are a technical writer documenting the %q module of a software project.", input.Module.ID),
	}

	if input.ArchSummary != nil {
		sections = append(sections, buildArchitectureBlock(input.ArchSummary))
		sections = append(sections, buildModuleRoleBlock(input.Module, input.NeighborSummaries))
	}

	sections = append(sections, "## Code skeleton", skeleton)
	if sharedBlock != "" {
		sections = append(sections, sharedBlock)
	}
	sections = append(sections,
		"## Instructions",
		"- Write a Markdown document with sections: Overview, Key Types, Key Functions, Usage Notes.",
		"- For every function reference, include a source link: [FuncName](path/to/file.go#L{line}).",
		"- Do NOT describe shared modules listed above — link to them using the format shown.",
		"- Use clear, concise technical prose. Avoid repeating the function signature verbatim.",
	)

	return strings.Join(sections, "\n\n")
}

func buildSharedModulesBlock(input AgentInput) string {
	if len(input.Module.DependsOnShared) == 0 {
		return ""
	}

	sections := []string{"## Shared modules (do not re-describe — link only)"}
	for _, moduleID := range input.Module.DependsOnShared {
		summary, ok := input.SharedContext[moduleID]
		if !ok {
			continue
		}

		keyFunctionNames := make([]string, 0, len(summary.KeyFunctions))
		for _, fn := range summary.KeyFunctions {
			keyFunctionNames = append(keyFunctionNames, fn.Name)
		}

		block := []string{fmt.Sprintf("### %s", moduleID)}
		if summary.Summary != "" {
			block = append(block, summary.Summary)
		}
		if len(keyFunctionNames) > 0 {
			block = append(block, fmt.Sprintf("Key functions: %s", strings.Join(keyFunctionNames, ", ")))
		}
		block = append(block, fmt.Sprintf("Reference: [See full docs](../shared/%s.md)", moduleID))

		sections = append(sections, strings.Join(block, "\n"))
	}

	if len(sections) == 1 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}

func buildArchitectureBlock(arch *store.ArchSummary) string {
	sections := []string{"## System Architecture"}
	sections = append(sections, fmt.Sprintf("Purpose: %s", arch.Purpose))
	if len(arch.Layers) > 0 {
		sections = append(sections, fmt.Sprintf("Layers: %s", strings.Join(arch.Layers, " > ")))
	}
	if arch.DataFlow != "" {
		sections = append(sections, fmt.Sprintf("Data flow: %s", arch.DataFlow))
	}
	return strings.Join(sections, "\n")
}

func buildModuleRoleBlock(m store.Module, summaries map[string]string) string {
	sections := []string{"## This Module's Role"}

	if len(m.DependsOnShared) > 0 {
		upstream := []string{"Upstream:"}
		for _, moduleID := range m.DependsOnShared {
			if desc, ok := summaries[moduleID]; ok {
				upstream = append(upstream, fmt.Sprintf("  %s: %s", moduleID, desc))
			} else {
				upstream = append(upstream, fmt.Sprintf("  %s", moduleID))
			}
		}
		sections = append(sections, strings.Join(upstream, "\n"))
	}

	if len(m.ReferencedBy) > 0 {
		downstream := []string{"Downstream:"}
		for _, moduleID := range m.ReferencedBy {
			if desc, ok := summaries[moduleID]; ok {
				downstream = append(downstream, fmt.Sprintf("  %s: %s", moduleID, desc))
			} else {
				downstream = append(downstream, fmt.Sprintf("  %s", moduleID))
			}
		}
		sections = append(sections, strings.Join(downstream, "\n"))
	}

	return strings.Join(sections, "\n")
}
