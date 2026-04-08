package planner

import (
	"fmt"

	"github.com/scalaview/wikismit/internal/agent"
	"github.com/scalaview/wikismit/internal/llm"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

// AgentType enumerates the different agent types the planner can create.
type AgentType string

const (
	AgentTypeExplore AgentType = "explore"
)

// PlannerAgent is the interface all planner agents must implement.
type PlannerAgent interface {
	Name() string
}

// NewPlannerAgent creates an agent of the specified type with config-driven dependencies.
func NewPlannerAgent(agentType AgentType, client llm.Client, cfg *configpkg.Config) (PlannerAgent, error) {
	switch agentType {
	case AgentTypeExplore:
		return newExploreAgent(client, cfg), nil
	default:
		return nil, fmt.Errorf("unknown planner agent type: %s", agentType)
	}
}

func newExploreAgent(client llm.Client, cfg *configpkg.Config) *agent.ExploreAgent {
	// Convert config filter values to planner's SkeletonFilterConfig
	plannerFilterCfg := SkeletonFilterConfig{
		MinFuncLines:  cfg.Analysis.Explore.MinFuncLines,
		MinCalledBy:   cfg.Analysis.Explore.MinCalledBy,
		MinImportance: cfg.Analysis.Explore.MinImportance,
	}

	// Wire skeleton builder: inject planner's BuildExploreSkeleton into agent
	// to break the circular import (agent can't import planner since planner imports agent).
	agent.SkeletonBuilderFunc = func(idx store.FileIndex, maxTokens int, filter *metrics.ImportanceFilter, agentCfg agent.SkeletonFilterConfig) string {
		// Convert agent's SkeletonFilterConfig to planner's local type
		plannerCfg := SkeletonFilterConfig{
			MinFuncLines:  agentCfg.MinFuncLines,
			MinCalledBy:   agentCfg.MinCalledBy,
			MinImportance: agentCfg.MinImportance,
		}
		return BuildExploreSkeleton(idx, maxTokens, filter, plannerCfg)
	}

	// Wire skeleton with summary builder: inject planner's BuildSkeletonOnlyWithSummary into agent
	// to break the circular import (agent can't import planner since planner imports agent).
	agent.SkeletonWithSummaryBuilderFunc = func(files []string, idx store.FileIndex, maxTokens int) string {
		return BuildSkeletonOnlyWithSummary(files, idx, maxTokens)
	}

	// Convert planner's SkeletonFilterConfig to agent's type
	agentFilterCfg := agent.SkeletonFilterConfig{
		MinFuncLines:  plannerFilterCfg.MinFuncLines,
		MinCalledBy:   plannerFilterCfg.MinCalledBy,
		MinImportance: plannerFilterCfg.MinImportance,
	}

	return agent.NewExploreAgent(client, &agent.ExploreConfig{
		Model:          cfg.LLM.PlannerModel,
		MaxTokens:      cfg.LLM.MaxTokens,
		Temperature:    cfg.LLM.Temperature,
		MaxRequests:    cfg.Analysis.Explore.MaxRequests,
		Language:       cfg.Language,
		SkeletonFilter: agentFilterCfg,
	})
}
