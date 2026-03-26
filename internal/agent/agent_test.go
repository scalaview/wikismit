package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func sampleRunAgentInput() AgentInput {
	return AgentInput{
		Module: store.Module{
			ID:    "auth",
			Files: []string{"internal/auth/jwt.go"},
		},
		FileIndex: store.FileIndex{
			"internal/auth/jwt.go": {
				Functions: []store.FunctionDecl{{
					Name:      "GenerateToken",
					Signature: "func GenerateToken() string",
					LineStart: 12,
					Exported:  true,
				}},
			},
		},
		Config: &configpkg.Config{
			Agent: configpkg.AgentConfig{SkeletonMaxTokens: 3000},
			LLM:   configpkg.LLMConfig{AgentModel: "agent-model", MaxTokens: 2048},
		},
	}
}

func TestRunAgentReturnsModuleDocOnSuccess(t *testing.T) {
	input := sampleRunAgentInput()
	client := llm.NewMockClient("# Auth module")

	got := runAgent(context.Background(), input.Module, input, client)

	if got.ModuleID != "auth" {
		t.Fatalf("runAgent().ModuleID = %q, want auth", got.ModuleID)
	}
	if got.Content != "# Auth module" {
		t.Fatalf("runAgent().Content = %q, want %q", got.Content, "# Auth module")
	}
	if got.Err != nil {
		t.Fatalf("runAgent().Err = %v, want nil", got.Err)
	}

	calls := client.Calls()
	if len(calls) != 1 {
		t.Fatalf("len(MockClient.Calls()) = %d, want 1", len(calls))
	}
	if calls[0].Model != "agent-model" {
		t.Fatalf("CompletionRequest.Model = %q, want agent-model", calls[0].Model)
	}
}

func TestRunAgentReturnsModuleDocErrorOnFailure(t *testing.T) {
	input := sampleRunAgentInput()
	client := llm.NewMockClient().WithErrors(errors.New("boom"))

	got := runAgent(context.Background(), input.Module, input, client)

	if got.ModuleID != "auth" {
		t.Fatalf("runAgent().ModuleID = %q, want auth", got.ModuleID)
	}
	if got.Content != "" {
		t.Fatalf("runAgent().Content = %q, want empty", got.Content)
	}
	if got.Err == nil {
		t.Fatal("runAgent().Err = nil, want error")
	}
}

func TestRunAgentLogsCompletionTiming(t *testing.T) {
	input := sampleRunAgentInput()
	client := llm.NewMockClient("# Auth module")
	var messages []string

	got := runAgentWithLogger(context.Background(), input.Module, input, client, func(level, msg string, fields ...any) {
		messages = append(messages, level+":"+msg)
	})

	if got.Err != nil {
		t.Fatalf("runAgentWithLogger().Err = %v, want nil", got.Err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if !strings.Contains(messages[0], "INFO:Phase 4: module auth completed in") {
		t.Fatalf("log message = %q, want completion timing", messages[0])
	}
}

func TestFormatPhase4SummaryIncludesFailuresOnlyWhenPresent(t *testing.T) {
	withFailures := formatPhase4Summary(3, []ModuleDoc{{ModuleID: "billing", Err: errors.New("boom")}})
	if !strings.Contains(withFailures, "Phase 4 complete: 2/3 modules documented") {
		t.Fatalf("summary = %q, want success count", withFailures)
	}
	if !strings.Contains(withFailures, "Failed modules:") {
		t.Fatalf("summary = %q, want failed modules section", withFailures)
	}
	if !strings.Contains(withFailures, "billing: boom") {
		t.Fatalf("summary = %q, want failure detail", withFailures)
	}

	withoutFailures := formatPhase4Summary(2, nil)
	if !strings.Contains(withoutFailures, "Phase 4 complete: 2/2 modules documented") {
		t.Fatalf("summary = %q, want all-success count", withoutFailures)
	}
	if strings.Contains(withoutFailures, "Failed modules:") {
		t.Fatalf("summary = %q, should not include failure section", withoutFailures)
	}
}

func TestPhase4PartialFailureWritesOnlySuccessfulModuleDocs(t *testing.T) {
	artifactsDir := t.TempDir()
	modules := []store.Module{{ID: "auth"}, {ID: "billing"}}
	input := sampleRunAgentInput()
	input.Config.ArtifactsDir = artifactsDir
	client := llm.NewMockClient("# Auth module").WithErrors(nil, errors.New("boom"))

	err := Run(context.Background(), modules, input, client, artifactsDir, 1)
	if err == nil {
		t.Fatal("Run() error = nil, want partial-failure error")
	}
	if !strings.Contains(err.Error(), "billing") {
		t.Fatalf("Run() error = %q, want billing in failure summary", err.Error())
	}

	if _, err := os.Stat(filepath.Join(artifactsDir, "module_docs", "auth.md")); err != nil {
		t.Fatalf("auth module doc missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "module_docs", "billing.md")); !os.IsNotExist(err) {
		t.Fatalf("billing module doc stat error = %v, want not exist", err)
	}
}
