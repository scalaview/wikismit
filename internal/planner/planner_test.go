package planner

import (
	"context"
	"strings"
	"testing"
	"time"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func samplePlannerConfig(t *testing.T) *configpkg.Config {
	t.Helper()
	return &configpkg.Config{
		RepoPath:     t.TempDir(),
		ArtifactsDir: t.TempDir(),
		LLM: configpkg.LLMConfig{
			PlannerModel: "planner-test-model",
			MaxTokens:    512,
			Temperature:  0.2,
		},
		Analysis: configpkg.AnalysisConfig{
			SharedModuleThreshold: 3,
		},
		Agent: configpkg.AgentConfig{
			SkeletonMaxTokens: 1_000,
		},
	}
}

func samplePlannerIndex() store.FileIndex {
	return store.FileIndex{
		"internal/auth/jwt.go": {
			Functions: []store.FunctionDecl{{
				Name:      "GenerateToken",
				Signature: "func GenerateToken() string",
				LineStart: 10,
				Exported:  true,
			}},
		},
	}
}

func samplePlannerGraph() store.DepGraph {
	return store.DepGraph{
		"internal/auth/jwt.go": {},
	}
}

func TestBuildPlannerPromptIncludesRulesThresholdAndSkeleton(t *testing.T) {
	skeleton := "// === internal/auth/jwt.go ===\nfunc GenerateToken() string  // internal/auth/jwt.go:10"

	got := buildPlannerPrompt(skeleton, 3)

	for _, want := range []string{
		"You are a software architect.",
		"3+ modules",
		"Respond ONLY with valid JSON.",
		"referenced_by",
		skeleton,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildPlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestRunPlannerSucceedsWithValidJSONResponse(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`)

	got, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.Modules[0].ID != "auth" {
		t.Fatalf("RunPlanner() module ID = %q, want auth", got.Modules[0].ID)
	}
	if client.CallCount() != 1 {
		t.Fatalf("MockClient.CallCount() = %d, want 1", client.CallCount())
	}
}

func TestRunPlannerRetriesAfterJSONParseFailure(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
	)

	got, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.Modules[0].ID != "auth" {
		t.Fatalf("RunPlanner() module ID = %q, want auth", got.Modules[0].ID)
	}
	if client.CallCount() != 2 {
		t.Fatalf("MockClient.CallCount() = %d, want 2", client.CallCount())
	}
}

func TestRunPlannerFailsAfterThreeInvalidResponses(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(`not-json`, `still-not-json`, `{"modules":[}`)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if client.CallCount() != 3 {
		t.Fatalf("MockClient.CallCount() = %d, want 3", client.CallCount())
	}
	if !strings.Contains(err.Error(), "parse nav plan") {
		t.Fatalf("RunPlanner() error = %v, want parse nav plan context", err)
	}
}

func TestRunPlannerRejectsMissingFileAssignments(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[]}`,
		`{"modules":[]}`,
		`{"modules":[]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "missing file assignment") {
		t.Fatalf("RunPlanner() error = %v, want missing file assignment context", err)
	}
}

func TestRunPlannerRejectsDuplicateFileAssignments(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"dup","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"dup","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"dup","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "duplicate file assignment") {
		t.Fatalf("RunPlanner() error = %v, want duplicate file assignment context", err)
	}
}

func TestRunPlannerRejectsInvalidOwnerValue(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"planner"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"planner"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"planner"}]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid owner") {
		t.Fatalf("RunPlanner() error = %v, want invalid owner context", err)
	}
}

func TestRunPlannerSetsGeneratedAtOnSuccess(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`)

	got, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.GeneratedAt.Equal(time.Time{}) {
		t.Fatal("RunPlanner() GeneratedAt is zero, want non-zero time")
	}
}
