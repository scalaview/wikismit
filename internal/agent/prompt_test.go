package agent

import (
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func sampleAgentConfig() *configpkg.Config {
	return &configpkg.Config{
		Agent: configpkg.AgentConfig{
			SkeletonMaxTokens: 3000,
		},
	}
}

func sampleAgentInput() AgentInput {
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
		`You are a technical writer documenting the "auth" module of a software project.`,
		"## Code skeleton",
		"func GenerateToken() string  // internal/auth/jwt.go:12",
		"Write a Markdown document with sections: Overview, Key Types, Key Functions, Usage Notes.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildAgentPrompt() missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "## Shared modules") {
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
			KeyFunctions: []store.KeyFunction{{
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
		if !strings.Contains(got, want) {
			t.Fatalf("BuildAgentPrompt() missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "Error helpers shared across modules.") {
		t.Fatalf("BuildAgentPrompt() unexpectedly included undeclared shared summary:\n%s", got)
	}
}

func TestBuildAgentPromptIncludesCitationFormatInstruction(t *testing.T) {
	got := BuildAgentPrompt(sampleAgentInput())

	want := "[FuncName](path/to/file.go#L{line})"
	if !strings.Contains(got, want) {
		t.Fatalf("BuildAgentPrompt() missing citation format instruction %q:\n%s", want, got)
	}
}

func TestBuildAgentPromptIncludesSharedOwnershipConstraint(t *testing.T) {
	got := BuildAgentPrompt(sampleAgentInput())

	want := "Do NOT describe shared modules"
	if !strings.Contains(got, want) {
		t.Fatalf("BuildAgentPrompt() missing ownership constraint %q:\n%s", want, got)
	}
}
