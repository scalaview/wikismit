package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNavPlanRoundtripWithArchitectureSummary(t *testing.T) {
	original := NavPlan{
		GeneratedAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
		Modules: []Module{
			{ID: "auth", Shared: false, Owner: "agent"},
		},
		ArchitectureSummary: &ArchSummary{
			Purpose:    "Authentication and authorization service",
			Layers:     []string{"API", "Business Logic", "Data"},
			DataFlow:   "Request -> Middleware -> Handler -> Store",
			KeyModules: []string{"auth", "store"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded NavPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ArchitectureSummary == nil {
		t.Fatal("ArchitectureSummary is nil after roundtrip")
	}
	if decoded.ArchitectureSummary.Purpose != original.ArchitectureSummary.Purpose {
		t.Fatalf("Purpose = %q, want %q", decoded.ArchitectureSummary.Purpose, original.ArchitectureSummary.Purpose)
	}
	if len(decoded.ArchitectureSummary.Layers) != 3 {
		t.Fatalf("Layers length = %d, want 3", len(decoded.ArchitectureSummary.Layers))
	}
	if decoded.ArchitectureSummary.DataFlow != original.ArchitectureSummary.DataFlow {
		t.Fatalf("DataFlow = %q, want %q", decoded.ArchitectureSummary.DataFlow, original.ArchitectureSummary.DataFlow)
	}
}

func TestNavPlanBackwardCompatWithoutArchitectureSummary(t *testing.T) {
	// JSON without architecture_summary should deserialize cleanly
	input := `{
		"generated_at": "2026-04-02T12:00:00Z",
		"modules": [{"id": "auth", "files": ["auth.go"], "shared": false, "owner": "agent"}]
	}`

	var plan NavPlan
	if err := json.Unmarshal([]byte(input), &plan); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if plan.ArchitectureSummary != nil {
		t.Fatal("ArchitectureSummary should be nil when not present in JSON")
	}
	if len(plan.Modules) != 1 {
		t.Fatalf("Modules length = %d, want 1", len(plan.Modules))
	}
}

func TestArchSummaryOmitsEmptyFields(t *testing.T) {
	summary := ArchSummary{Purpose: "A service"}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if string(data) != `{"purpose":"A service"}` {
		t.Fatalf("unexpected JSON: %s", data)
	}
}
