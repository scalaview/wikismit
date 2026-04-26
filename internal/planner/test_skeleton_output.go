package planner

import (
	"fmt"
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

// This test is for manual inspection of skeleton output
func TestSkeletonOutputForInspection(t *testing.T) {
	idx := store.FileIndex{
		"internal/auth/jwt.go": {
			Functions: []*store.FunctionDecl{{
				Name:      "GenerateToken",
				Signature: "func GenerateToken() string",
				LineStart: 10,
				Exported:  true,
			}},
			Types: []*store.TypeDecl{{
				Name:      "Claims",
				Kind:      "struct",
				LineStart: 2,
				Exported:  true,
			}},
			Imports: []*store.Import{{
				Path:         "github.com/example/pkg/crypto",
				Internal:     true,
				ResolvedPath: "pkg/crypto/hash.go",
			}},
		},
		"internal/util/empty.go": {}, // Empty file - only header expected
		"pkg/only_external.go": {
			Types: []*store.TypeDecl{{
				Name:      "ExternalType",
				Kind:      "struct",
				LineStart: 1,
				Exported:  true,
			}},
			Imports: []*store.Import{{
				Path:     "fmt",
				Internal: false,
			}},
		},
		"pkg/no_exports.go": {
			Functions: []*store.FunctionDecl{{
				Name:      "internalFunc",
				Signature: "func internalFunc()",
				LineStart: 1,
				Exported:  false,
			}},
		},
	}
	
	result := BuildPlannerSkeleton(idx, 10000)
	fmt.Println("=== SKELETON OUTPUT ===")
	fmt.Println(result)
	fmt.Println("=== END ===")

	// Verify expectations
	if !containsLine(result, "FILE: internal/auth/jwt.go") {
		t.Error("missing jwt.go FILE header")
	}
	if !containsLine(result, "QUERYABLE_FUNCTIONS") {
		t.Error("missing QUERYABLE_FUNCTIONS section")
	}
	if !containsLine(result, "  internal/auth/jwt.go#GenerateToken") {
		t.Error("missing function ID")
	}
	if !containsLine(result, "TYPES") {
		t.Error("missing TYPES section")
	}
	if !containsLine(result, "  Claims struct") {
		t.Error("missing Claims type")
	}
	if !containsLine(result, "INTERNAL_IMPORTS") {
		t.Error("missing INTERNAL_IMPORTS section")
	}
	if !containsLine(result, "  pkg/crypto/hash.go") {
		t.Error("missing internal import")
	}

	if !containsLine(result, "FILE: internal/util/empty.go") {
		t.Error("missing empty.go FILE header")
	}

	if !containsLine(result, "FILE: pkg/only_external.go") {
		t.Error("missing only_external.go FILE header")
	}
	if !containsLine(result, "  ExternalType struct") {
		t.Error("missing ExternalType")
	}

	if !containsLine(result, "FILE: pkg/no_exports.go") {
		t.Error("missing no_exports.go FILE header")
	}
}

func containsLine(s, substr string) bool {
	lines := splitLines(s)
	for _, line := range lines {
		if line == substr {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == '\n' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
