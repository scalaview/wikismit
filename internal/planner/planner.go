package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func RunPlanner(ctx context.Context, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (*store.NavPlan, error) {
	_ = graph

	skeleton := BuildFullSkeleton(idx, cfg.Agent.SkeletonMaxTokens)
	prompt := buildPlannerPrompt(skeleton, cfg.Analysis.SharedModuleThreshold)

	parseErrors := make([]string, 0, 3)
	for attempt := 0; attempt < 3; attempt++ {
		response, err := client.Complete(ctx, llm.CompletionRequest{
			Model:       cfg.LLM.PlannerModel,
			UserMsg:     prompt,
			MaxTokens:   cfg.LLM.MaxTokens,
			Temperature: cfg.LLM.Temperature,
		})
		if err != nil {
			return nil, err
		}

		var plan store.NavPlan
		if err := json.Unmarshal([]byte(response), &plan); err == nil {
			if err := validateNavPlan(plan, idx); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("attempt %d: %v", attempt+1, err))
				prompt = prompt + fmt.Sprintf("\n\nPrevious response failed validation: %v. Try again.", err)
				continue
			}
			plan.GeneratedAt = time.Now().UTC()
			return &plan, nil
		} else {
			parseErrors = append(parseErrors, fmt.Sprintf("attempt %d: parse nav plan: %v", attempt+1, err))
			prompt = prompt + fmt.Sprintf("\n\nPrevious response failed JSON parse: %v. Try again.", err)
		}
	}

	return nil, fmt.Errorf("%s", strings.Join(parseErrors, "; "))
}

func validateNavPlan(plan store.NavPlan, idx store.FileIndex) error {
	seen := make(map[string]string, len(idx))
	for _, module := range plan.Modules {
		if module.Owner != "agent" && module.Owner != "shared_preprocessor" {
			return fmt.Errorf("invalid owner %q for module %q", module.Owner, module.ID)
		}
		for _, file := range module.Files {
			if owner, exists := seen[file]; exists {
				return fmt.Errorf("duplicate file assignment for %q in modules %q and %q", file, owner, module.ID)
			}
			seen[file] = module.ID
		}
	}

	for file := range idx {
		if _, ok := seen[file]; !ok {
			return fmt.Errorf("missing file assignment for %q", file)
		}
	}

	return nil
}
