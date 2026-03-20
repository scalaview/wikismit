package store

import (
	"errors"
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

func TestReadReturnsErrArtifactNotFound(t *testing.T) {
	_, err := ReadFileIndex(t.TempDir())
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("error = %v, want ErrArtifactNotFound", err)
	}
}
