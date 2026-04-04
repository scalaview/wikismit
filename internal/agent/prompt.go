package agent

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/scalaview/wikismit/internal/agent/prompt"
	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/internal/planner"
)

var logger = logpkg.New(false)

type AgentPromptData struct {
	SystemMsg string
	UserMsg   string
}

func BuildAgentPrompt(input *AgentInput) *AgentPromptData {
	skeleton := planner.BuildSkeletonOnlyWithSummary(input.Module.Files, input.FileIndex, input.Config.Agent.SkeletonMaxTokens)
	sharedBlock := buildSharedModulesBlock(input)

	var sysBuf bytes.Buffer
	if err := prompt.ModuleSystemPromptTmp.Execute(&sysBuf, &prompt.ModuleSystemPromptData{
		RepoType: "Golang", //TODO: make this dynamic based on input.Config.Language
		RepoName: input.Module.ID,
		Language: input.Config.Language,
	}); err != nil {
		logger.Fault("execute module system prompt: %v", err)
	}

	var userBuf bytes.Buffer
	if err := prompt.ModuleUserPromptTmp.Execute(&userBuf, &prompt.ModuleUserPromptData{
		ModuleID:    input.Module.ID,
		Skeleton:    skeleton,
		SharedBlock: sharedBlock,
		Language:    input.Config.Language,
	}); err != nil {
		logger.Fault("execute module user prompt: %v", err)
	}

	return &AgentPromptData{
		SystemMsg: sysBuf.String(),
		UserMsg:   userBuf.String(),
	}
}

func buildSharedModulesBlock(input *AgentInput) string {
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
