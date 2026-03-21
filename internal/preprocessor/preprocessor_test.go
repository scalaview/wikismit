package preprocessor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func sampleGroundingIndex() store.FileIndex {
	return store.FileIndex{
		"pkg/logger/logger.go": {
			Functions: []store.FunctionDecl{{
				Name:      "New",
				Signature: "func New() Logger",
				LineStart: 18,
				Exported:  true,
			}},
		},
	}
}

func sampleSharedContext() store.SharedContext {
	return store.SharedContext{
		"errors": {
			Summary: "Shared error helpers for auth and logger.",
		},
	}
}

func TestBuildSharedPromptIncludesSkeletonAndJSONContract(t *testing.T) {
	skeleton := "// === pkg/logger/logger.go ===\nfunc New() Logger  // pkg/logger/logger.go:18"

	got := buildSharedPrompt("logger", skeleton, nil)

	for _, want := range []string{
		"You are documenting the shared module \"logger\".",
		"Respond ONLY with valid JSON:",
		"\"summary\"",
		skeleton,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildSharedPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestBuildSharedPromptInjectsAlreadySummarisedDependencies(t *testing.T) {
	skeleton := "// === pkg/logger/logger.go ==="

	got := buildSharedPrompt("logger", skeleton, sampleSharedContext())

	if !strings.Contains(got, "The following shared modules are used by this module.") {
		t.Fatalf("buildSharedPrompt() missing dependency context block:\n%s", got)
	}
	if !strings.Contains(got, "- errors: Shared error helpers for auth and logger.") {
		t.Fatalf("buildSharedPrompt() missing summarised dependency:\n%s", got)
	}
}

func TestGroundSharedSummaryRefsUsesFileIndexLineNumbers(t *testing.T) {
	summary := store.SharedSummary{
		KeyFunctions: []store.KeyFunction{{
			Name:      "New",
			Signature: "func New() Logger",
			Ref:       "some/other/place.go#L999",
		}},
	}

	got := groundSharedSummaryRefs(summary, []string{"pkg/logger/logger.go"}, sampleGroundingIndex())

	if got.KeyFunctions[0].Ref != "pkg/logger/logger.go#L18" {
		t.Fatalf("groundSharedSummaryRefs() ref = %q, want pkg/logger/logger.go#L18", got.KeyFunctions[0].Ref)
	}
	if len(got.SourceRefs) != 1 || got.SourceRefs[0] != "pkg/logger/logger.go#L18" {
		t.Fatalf("groundSharedSummaryRefs() SourceRefs = %v, want [pkg/logger/logger.go#L18]", got.SourceRefs)
	}
}

func TestGroundSharedSummaryRefsKeepsUnknownRefAndWarns(t *testing.T) {
	summary := store.SharedSummary{
		KeyFunctions: []store.KeyFunction{{
			Name:      "Missing",
			Signature: "func Missing()",
			Ref:       "pkg/logger/logger.go#L999",
		}},
	}

	got := groundSharedSummaryRefs(summary, []string{"pkg/logger/logger.go"}, sampleGroundingIndex())

	if got.KeyFunctions[0].Ref != "pkg/logger/logger.go#L999" {
		t.Fatalf("groundSharedSummaryRefs() ref = %q, want unchanged fallback ref", got.KeyFunctions[0].Ref)
	}
	if len(got.SourceRefs) != 1 || got.SourceRefs[0] != "pkg/logger/logger.go#L999" {
		t.Fatalf("groundSharedSummaryRefs() SourceRefs = %v, want fallback ref", got.SourceRefs)
	}
}

func samplePreprocessorConfig(t *testing.T) *configpkg.Config {
	t.Helper()
	return &configpkg.Config{
		RepoPath:     t.TempDir(),
		ArtifactsDir: t.TempDir(),
		Analysis: configpkg.AnalysisConfig{
			SharedModuleThreshold: 3,
		},
		Agent: configpkg.AgentConfig{
			SkeletonMaxTokens: 3000,
		},
	}
}

func samplePreprocessorIndex() store.FileIndex {
	return store.FileIndex{
		"pkg/errors/errors.go": {
			Functions: []store.FunctionDecl{{
				Name:      "Wrap",
				Signature: "func Wrap(err error) error",
				LineStart: 11,
				Exported:  true,
			}},
		},
		"pkg/logger/logger.go": {
			Functions: []store.FunctionDecl{{
				Name:      "New",
				Signature: "func New() Logger",
				LineStart: 18,
				Exported:  true,
			}},
		},
	}
}

func samplePreprocessorPlan() *store.NavPlan {
	return &store.NavPlan{
		Modules: []store.Module{
			{ID: "errors", Files: []string{"pkg/errors/errors.go"}, Shared: true, Owner: "shared_preprocessor"},
			{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
		},
	}
}

func sampleNoSharedPlan() *store.NavPlan {
	return &store.NavPlan{
		Modules: []store.Module{{
			ID:     "auth",
			Files:  []string{"internal/auth/jwt.go"},
			Shared: false,
			Owner:  "agent",
		}},
	}
}

func samplePreprocessorGraph() store.DepGraph {
	return store.DepGraph{
		"pkg/errors/errors.go": {},
		"pkg/logger/logger.go": {"pkg/errors/errors.go"},
	}
}

func TestRunPreprocessorWritesSharedContextInTopologicalOrder(t *testing.T) {
	cfg := samplePreprocessorConfig(t)
	client := llm.NewMockClient(
		`{"summary":"Shared error helpers.","key_types":["Error"],"key_functions":[{"name":"Wrap","signature":"func Wrap(err error) error","ref":"wrong.go#L1"}]}`,
		`{"summary":"Structured logger built on shared errors.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() Logger","ref":"wrong.go#L2"}]}`,
	)

	got, err := RunPreprocessor(context.Background(), samplePreprocessorPlan(), samplePreprocessorIndex(), samplePreprocessorGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPreprocessor() error = %v", err)
	}
	if client.CallCount() != 2 {
		t.Fatalf("MockClient.CallCount() = %d, want 2", client.CallCount())
	}
	if got["errors"].KeyFunctions[0].Ref != "pkg/errors/errors.go#L11" {
		t.Fatalf("errors ref = %q, want grounded ref", got["errors"].KeyFunctions[0].Ref)
	}
	if got["logger"].KeyFunctions[0].Ref != "pkg/logger/logger.go#L18" {
		t.Fatalf("logger ref = %q, want grounded ref", got["logger"].KeyFunctions[0].Ref)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArtifactsDir, "shared_context.json")); err != nil {
		t.Fatalf("shared_context.json missing: %v", err)
	}
	calls := client.Calls()
	if strings.Contains(calls[0].UserMsg, "- errors: Shared error helpers.") {
		t.Fatalf("first prompt unexpectedly contained dependency summary:\n%s", calls[0].UserMsg)
	}
	if !strings.Contains(calls[1].UserMsg, "- errors: Shared error helpers.") {
		t.Fatalf("second prompt missing dependency summary:\n%s", calls[1].UserMsg)
	}
}

func TestRunPreprocessorSkipsLLMCallsWhenNoSharedModulesExist(t *testing.T) {
	cfg := samplePreprocessorConfig(t)
	client := llm.NewMockClient()

	got, err := RunPreprocessor(context.Background(), sampleNoSharedPlan(), samplePreprocessorIndex(), samplePreprocessorGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPreprocessor() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("RunPreprocessor() = %v, want empty context", got)
	}
	if client.CallCount() != 0 {
		t.Fatalf("MockClient.CallCount() = %d, want 0", client.CallCount())
	}
}

func TestRunPreprocessorReturnsErrorOnInvalidSharedSummaryJSON(t *testing.T) {
	cfg := samplePreprocessorConfig(t)
	client := llm.NewMockClient(`{"summary":`)

	_, err := RunPreprocessor(context.Background(), samplePreprocessorPlan(), samplePreprocessorIndex(), samplePreprocessorGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPreprocessor() error = nil, want error")
	}
	if _, statErr := os.Stat(filepath.Join(cfg.ArtifactsDir, "shared_context.json")); !os.IsNotExist(statErr) {
		t.Fatalf("shared_context.json stat error = %v, want not exist", statErr)
	}
}

func TestRunPreprocessorUsesConfiguredPreprocessorModel(t *testing.T) {
	cfg := samplePreprocessorConfig(t)
	cfg.LLM.PlannerModel = "planner-model"
	cfg.LLM.PreprocessorModel = "preprocessor-model"
	client := llm.NewMockClient(
		`{"summary":"Shared error helpers.","key_types":["Error"],"key_functions":[{"name":"Wrap","signature":"func Wrap(err error) error","ref":"wrong.go#L1"}]}`,
		`{"summary":"Structured logger built on shared errors.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() Logger","ref":"wrong.go#L2"}]}`,
	)

	_, err := RunPreprocessor(context.Background(), samplePreprocessorPlan(), samplePreprocessorIndex(), samplePreprocessorGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPreprocessor() error = %v", err)
	}
	calls := client.Calls()
	if len(calls) != 2 {
		t.Fatalf("len(MockClient.Calls()) = %d, want 2", len(calls))
	}
	for _, call := range calls {
		if call.Model != "preprocessor-model" {
			t.Fatalf("CompletionRequest.Model = %q, want preprocessor-model", call.Model)
		}
	}
}

func TestRunPreprocessorFallsBackToPlannerModel(t *testing.T) {
	cfg := samplePreprocessorConfig(t)
	cfg.LLM.PlannerModel = "planner-model"
	client := llm.NewMockClient(
		`{"summary":"Shared error helpers.","key_types":["Error"],"key_functions":[{"name":"Wrap","signature":"func Wrap(err error) error","ref":"wrong.go#L1"}]}`,
		`{"summary":"Structured logger built on shared errors.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() Logger","ref":"wrong.go#L2"}]}`,
	)

	_, err := RunPreprocessor(context.Background(), samplePreprocessorPlan(), samplePreprocessorIndex(), samplePreprocessorGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPreprocessor() error = %v", err)
	}
	calls := client.Calls()
	if len(calls) != 2 {
		t.Fatalf("len(MockClient.Calls()) = %d, want 2", len(calls))
	}
	for _, call := range calls {
		if call.Model != "planner-model" {
			t.Fatalf("CompletionRequest.Model = %q, want planner-model", call.Model)
		}
	}
}
