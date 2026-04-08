package agent

import (
	"fmt"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func init() {
	// Initialize function pointer for tests (normally set by planner's agent factory)
	SkeletonWithSummaryBuilderFunc = func(files []string, idx store.FileIndex, maxTokens int) string {
		// Simple test implementation that mimics planner.BuildSkeletonOnlyWithSummary format
		var result strings.Builder
		for _, file := range files {
			if entry, ok := idx[file]; ok {
				result.WriteString(fmt.Sprintf("<file: %s>\n", file))
				for _, fn := range entry.Functions {
					result.WriteString(fn.Signature)
					result.WriteString(fmt.Sprintf("  // %d,%d", fn.LineStart, fn.LineEnd))
					if fn.Summary != "" {
						result.WriteString("\ndescription:")
						result.WriteString(fn.Summary)
					}
					result.WriteString("\n")
				}
				result.WriteString("</file>\n")
			}
		}
		return result.String()
	}
}

func sampleAgentConfig() *configpkg.Config {
	return &configpkg.Config{
		Agent: &configpkg.AgentConfig{
			SkeletonMaxTokens: 3000,
		},
	}
}

func sampleAgentInput() *AgentInput {
	return &AgentInput{
		Module: &store.Module{
			ID:    "auth",
			Files: []string{"internal/auth/jwt.go"},
		},
		FileIndex: store.FileIndex{
			"internal/auth/jwt.go": {
				Functions: []*store.FunctionDecl{{
					Name:      "GenerateToken",
					Signature: "func GenerateToken() string",
					LineStart: 12,
					Exported:  true,
				}},
			},
		},
		SharedContext: store.SharedContext{
			"logger": {
				Summary: "Shared logger helpers.",
			},
		},
		Config: sampleAgentConfig(),
	}
}

func TestBuildAgentPromptUsesAgentInputModuleAndArtifacts(t *testing.T) {
	input := sampleAgentInput()

	if input.Module.ID != "auth" {
		t.Fatalf("AgentInput.Module.ID = %q, want auth", input.Module.ID)
	}
	if len(input.FileIndex) != 1 {
		t.Fatalf("len(AgentInput.FileIndex) = %d, want 1", len(input.FileIndex))
	}
	if input.SharedContext["logger"].Summary == "" {
		t.Fatal("AgentInput.SharedContext logger summary should not be empty")
	}
	if input.Config == nil {
		t.Fatal("AgentInput.Config should not be nil")
	}
}

func TestBuildAgentPromptOmitsSharedContextWhenModuleHasNoSharedDeps(t *testing.T) {
	input := sampleAgentInput()
	input.Module.DependsOnShared = nil

	got := BuildAgentPrompt(input)

	for _, want := range []string{
		"You are an expert technical writer and software architect.",
		"The module id is auth",
		"func GenerateToken() string  // 12,0",
		"**Introduction:** Start with a concise introduction",
	} {
		if !strings.Contains(got.UserMsg, want) {
			t.Fatalf("BuildAgentPrompt() missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got.UserMsg, "## Shared modules") {
		t.Fatalf("BuildAgentPrompt() unexpectedly included shared modules block:\n%s", got)
	}
}

func TestBuildAgentPromptInjectsDeclaredSharedDependenciesOnly(t *testing.T) {
	input := sampleAgentInput()
	input.Module.DependsOnShared = []string{"logger"}
	input.SharedContext = store.SharedContext{
		"logger": {
			Summary:  "Structured logger wrapping zerolog.",
			KeyTypes: []string{"Logger"},
			KeyFunctions: []*store.KeyFunction{{
				Name:      "New",
				Signature: "func New() Logger",
				Ref:       "pkg/logger/logger.go#L18",
			}},
		},
		"errors": {
			Summary: "Error helpers shared across modules.",
		},
	}

	got := BuildAgentPrompt(input)

	for _, want := range []string{
		"## Shared modules (do not re-describe — link only)",
		"### logger",
		"Structured logger wrapping zerolog.",
		"Key functions: New",
		"Reference: [See full docs](../shared/logger.md)",
	} {
		if !strings.Contains(got.UserMsg, want) {
			t.Fatalf("BuildAgentPrompt() missing %q:\n%s", want, got.UserMsg)
		}
	}

	if strings.Contains(got.UserMsg, "Error helpers shared across modules.") {
		t.Fatalf("BuildAgentPrompt() unexpectedly included undeclared shared summary:\n%s", got.UserMsg)
	}
}

func TestBuildAgentPromptIncludesCitationFormatInstruction(t *testing.T) {
	got := BuildAgentPrompt(sampleAgentInput())

	want := "Sources: [filename.ext:start_line-end_line]()"
	if !strings.Contains(got.UserMsg, want) {
		t.Fatalf("BuildAgentPrompt() missing citation format instruction %q:\n%s", want, got.UserMsg)
	}
}

func TestBuildAgentPromptIncludesSharedOwnershipConstraint(t *testing.T) {
	input := sampleAgentInput()
	input.Module.DependsOnShared = []string{"logger"}
	input.SharedContext = store.SharedContext{
		"logger": {
			Summary: "Shared logger helpers.",
		},
	}

	got := BuildAgentPrompt(input)

	want := "do not re-describe"
	if !strings.Contains(got.UserMsg, want) {
		t.Fatalf("BuildAgentPrompt() missing ownership constraint %q:\n%s", want, got.UserMsg)
	}
}
