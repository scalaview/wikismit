package preprocessor

import (
	"strings"
	"testing"

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
