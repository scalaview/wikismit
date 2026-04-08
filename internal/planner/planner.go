package planner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/scalaview/wikismit/internal/agent"
	"github.com/scalaview/wikismit/internal/metrics"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

func RunPlanner(ctx context.Context, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (*store.NavPlan, error) {
	_ = graph

	plannerLogger := logger
	if plannerLogger == nil {
		plannerLogger = logpkg.New(cfg.Verbose)
		logger = plannerLogger
		defer func() {
			logger = nil
		}()
	}

	// Load function metrics if available, use importance-annotated skeleton
	metricsData, metricsErr := store.ReadMetrics(cfg.ArtifactsDir)
	var skeleton string
	if metricsErr == nil && len(metricsData) > 0 {
		filter := metrics.NewImportanceFilter(metricsData, 0)
		skeleton = BuildPlannerSkeletonWithImportance(idx, cfg.Agent.SkeletonMaxTokens, filter)
	} else {
		skeleton = BuildPlannerSkeleton(idx, cfg.Agent.SkeletonMaxTokens)
	}

	// Project structure exploration (optional, controlled by config)
	var explorationContext string
	if cfg.Analysis.Explore != nil && cfg.Analysis.Explore.Enabled {
		exploreAgent, err := NewPlannerAgent(AgentTypeExplore, client, cfg)
		if err != nil {
			plannerLogger.Warn("failed to create explore agent", "error", err)
		} else {
			metricsData, _ := store.ReadMetrics(cfg.ArtifactsDir)
			if ea, ok := exploreAgent.(*agent.ExploreAgent); ok {
				result, err := ea.Run(ctx, idx, nil, metricsData)
				if err != nil {
					plannerLogger.Warn("project exploration failed", "error", err)
				} else {
					explorationContext = buildExplorationContext(result)
				}
			}
		}
	}

	prompt := buildPlannerPrompt(skeleton, cfg.Analysis.SharedModuleThreshold) + explorationContext

	parseErrors := make([]string, 0, 3)
	for attempt := range 3 {
		plannerLogger.Debug("starting planner completion request",
			"skeleton_token_estimate", estimateTokens(skeleton),
			"prompt_length", len(prompt),
			"planner_attempt", attempt+1,
			"model", cfg.LLM.PlannerModel,
		)
		response, err := client.Complete(ctx, &llm.CompletionRequest{
			Model:       cfg.LLM.PlannerModel,
			UserMsg:     prompt,
			MaxTokens:   cfg.LLM.MaxTokens,
			Temperature: cfg.LLM.Temperature,
		})
		if err != nil {
			return nil, err
		}

		var plan store.NavPlan
		if err := llm.ParseJSON(response, &plan); err == nil {
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
	seenModuleIDs := make(map[string]struct{}, len(plan.Modules))
	for _, module := range plan.Modules {
		if module.ID == "" {
			return fmt.Errorf("empty module id")
		}
		if _, exists := seenModuleIDs[module.ID]; exists {
			return fmt.Errorf("duplicate module id %q", module.ID)
		}
		seenModuleIDs[module.ID] = struct{}{}
		if module.Owner != "agent" && module.Owner != "shared_preprocessor" {
			return fmt.Errorf("invalid owner %q for module %q", module.Owner, module.ID)
		}
		for _, file := range module.Files {
			if _, ok := idx[file]; !ok {
				return fmt.Errorf("file %q in module %q not found in file index", file, module.ID)
			}
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

func buildExplorationContext(result *agent.ExploreResult) string {
	var sb strings.Builder
	sb.WriteString("\n\n<project_exploration>\nThe following files/functions were identified as architecturally important:\n")
	for _, req := range result.Requests {
		sb.WriteString(fmt.Sprintf("- %s %s: %s\n", req.Type, req.Target, req.Reason))
	}
	sb.WriteString("</project_exploration>")
	return sb.String()
}
