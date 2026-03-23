package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleFileIndex() FileIndex {
	return FileIndex{
		"internal/auth/jwt.go": {
			Language:    "go",
			ContentHash: "sha256:abc",
			Functions: []FunctionDecl{{
				Name:      "GenerateToken",
				Signature: "func GenerateToken() string",
				LineStart: 10,
				LineEnd:   20,
				Exported:  true,
			}},
			Types: []TypeDecl{{
				Name:      "Claims",
				Kind:      "struct",
				LineStart: 1,
				Exported:  true,
			}},
			Imports: []Import{{Path: "internal/models", Internal: true}},
		},
	}
}

func sampleDepGraph() DepGraph {
	return DepGraph{
		"internal/auth/jwt.go": {"internal/models/user.go"},
	}
}

func sampleNavPlan() NavPlan {
	return NavPlan{
		GeneratedAt: time.Unix(1710000000, 0).UTC(),
		Modules: []Module{{
			ID:              "auth",
			Files:           []string{"internal/auth/jwt.go"},
			Shared:          false,
			Owner:           "agent",
			DependsOnShared: []string{"logger"},
		}},
	}
}

func sampleSharedContext() SharedContext {
	return SharedContext{
		"logger": {
			Summary:    "Structured logger wrapper",
			KeyTypes:   []string{"Logger"},
			SourceRefs: []string{"pkg/logger/logger.go#L1"},
			KeyFunctions: []KeyFunction{{
				Name:      "New",
				Signature: "func New() Logger",
				Ref:       "pkg/logger/logger.go#L18",
			}},
		},
	}
}

func sampleValidationReport() ValidationReport {
	return ValidationReport{
		GeneratedAt: time.Unix(1710001234, 0).UTC(),
		BrokenLinks: []BrokenLink{{
			SourceFile: "docs/modules/auth.md",
			LinkText:   "GenerateToken",
			LinkTarget: "internal/auth/jwt.md#generate-token",
			Line:       14,
		}},
		TotalLinks: 9,
		TotalFiles: 3,
	}
}

func TestWriteAndReadFileIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleFileIndex()

	if err := WriteFileIndex(dir, want); err != nil {
		t.Fatalf("WriteFileIndex() error = %v", err)
	}
	got, err := ReadFileIndex(dir)
	if err != nil {
		t.Fatalf("ReadFileIndex() error = %v", err)
	}
	if got["internal/auth/jwt.go"].ContentHash != want["internal/auth/jwt.go"].ContentHash {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteAndReadDepGraphRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleDepGraph()

	if err := WriteDepGraph(dir, want); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}
	got, err := ReadDepGraph(dir)
	if err != nil {
		t.Fatalf("ReadDepGraph() error = %v", err)
	}
	if len(got["internal/auth/jwt.go"]) != 1 || got["internal/auth/jwt.go"][0] != "internal/models/user.go" {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteAndReadNavPlanRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleNavPlan()

	if err := WriteNavPlan(dir, want); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	got, err := ReadNavPlan(dir)
	if err != nil {
		t.Fatalf("ReadNavPlan() error = %v", err)
	}
	if got.Modules[0].ID != want.Modules[0].ID {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteAndReadSharedContextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleSharedContext()

	if err := WriteSharedContext(dir, want); err != nil {
		t.Fatalf("WriteSharedContext() error = %v", err)
	}
	got, err := ReadSharedContext(dir)
	if err != nil {
		t.Fatalf("ReadSharedContext() error = %v", err)
	}
	if got["logger"].Summary != want["logger"].Summary {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteValidationReportRoundTripsJSON(t *testing.T) {
	dir := t.TempDir()
	want := sampleValidationReport()

	if err := WriteValidationReport(dir, want); err != nil {
		t.Fatalf("WriteValidationReport() error = %v", err)
	}

	path := filepath.Join(dir, "validation_report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var got ValidationReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.TotalLinks != want.TotalLinks {
		t.Fatalf("TotalLinks = %d, want %d", got.TotalLinks, want.TotalLinks)
	}
	if got.TotalFiles != want.TotalFiles {
		t.Fatalf("TotalFiles = %d, want %d", got.TotalFiles, want.TotalFiles)
	}
	if len(got.BrokenLinks) != 1 {
		t.Fatalf("len(BrokenLinks) = %d, want 1", len(got.BrokenLinks))
	}
	if got.BrokenLinks[0].LinkTarget != want.BrokenLinks[0].LinkTarget {
		t.Fatalf("LinkTarget = %q, want %q", got.BrokenLinks[0].LinkTarget, want.BrokenLinks[0].LinkTarget)
	}
}

func TestReadReturnsErrArtifactNotFound(t *testing.T) {
	_, err := ReadFileIndex(t.TempDir())
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("error = %v, want ErrArtifactNotFound", err)
	}
}
