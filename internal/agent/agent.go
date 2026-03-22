package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func runAgent(ctx context.Context, module store.Module, input AgentInput, client llm.Client) ModuleDoc {
	return runAgentWithLogger(ctx, module, input, client, nil)
}

func runAgentWithLogger(ctx context.Context, module store.Module, input AgentInput, client llm.Client, logf func(level, msg string, fields ...any)) ModuleDoc {
	requestInput := input
	requestInput.Module = module
	start := time.Now()

	content, err := client.Complete(ctx, llm.CompletionRequest{
		Model:     input.Config.LLM.AgentModel,
		UserMsg:   BuildAgentPrompt(requestInput),
		MaxTokens: input.Config.LLM.MaxTokens,
	})
	if err != nil {
		if logf != nil {
			logf("ERROR", fmt.Sprintf("Phase 4: module %s failed in %s: %v", module.ID, time.Since(start), err))
		}
		return ModuleDoc{ModuleID: module.ID, Err: err}
	}
	if logf != nil {
		logf("INFO", fmt.Sprintf("Phase 4: module %s completed in %s", module.ID, time.Since(start)))
	}

	return ModuleDoc{ModuleID: module.ID, Content: content}
}

func formatPhase4Summary(total int, failures []ModuleDoc) string {
	successCount := total - len(failures)
	sections := []string{fmt.Sprintf("Phase 4 complete: %d/%d modules documented", successCount, total)}
	if len(failures) == 0 {
		return sections[0]
	}

	sections = append(sections, "Failed modules:")
	for _, failure := range failures {
		sections = append(sections, fmt.Sprintf("  - %s: %v", failure.ModuleID, failure.Err))
	}

	return strings.Join(sections, "\n")
}
