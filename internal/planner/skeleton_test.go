package planner

import (
	"strings"
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func samplePlannerFileIndex() store.FileIndex {
	return store.FileIndex{
		"internal/auth/jwt.go": {
			Functions: []store.FunctionDecl{{
				Name:      "GenerateToken",
				Signature: "func GenerateToken() string",
				LineStart: 10,
				Exported:  true,
			}, {
				Name:      "generateTokenSecret",
				Signature: "func generateTokenSecret() string",
				LineStart: 18,
				Exported:  false,
			}},
			Types: []store.TypeDecl{{
				Name:      "Claims",
				Kind:      "struct",
				LineStart: 2,
				Exported:  true,
			}, {
				Name:      "tokenConfig",
				Kind:      "struct",
				LineStart: 6,
				Exported:  false,
			}},
		},
		"internal/auth/middleware.go": {
			Functions: []store.FunctionDecl{{
				Name:      "Middleware",
				Signature: "func Middleware()",
				LineStart: 5,
				Exported:  true,
			}},
		},
	}
}

func TestEstimateTokensUsesSimpleCharacterApproximation(t *testing.T) {
	const text = "12345678"

	got := estimateTokens(text)
	if got != 2 {
		t.Fatalf("estimateTokens() = %d, want 2", got)
	}
}

func TestBuildSkeletonIncludesAnnotatedFunctionAndTypeLines(t *testing.T) {
	idx := samplePlannerFileIndex()

	got := BuildSkeleton([]string{"internal/auth/jwt.go"}, idx, 10_000)

	if !strings.Contains(got, "func GenerateToken() string  // internal/auth/jwt.go:10") {
		t.Fatalf("BuildSkeleton() missing function annotation:\n%s", got)
	}
	if !strings.Contains(got, "type Claims struct  // internal/auth/jwt.go:2") {
		t.Fatalf("BuildSkeleton() missing type annotation:\n%s", got)
	}
}

func TestBuildSkeletonSeparatesFilesWithHeaders(t *testing.T) {
	idx := samplePlannerFileIndex()

	got := BuildSkeleton([]string{"internal/auth/middleware.go", "internal/auth/jwt.go"}, idx, 10_000)

	first := strings.Index(got, "// === internal/auth/jwt.go ===")
	second := strings.Index(got, "// === internal/auth/middleware.go ===")
	if first == -1 || second == -1 {
		t.Fatalf("BuildSkeleton() missing file headers:\n%s", got)
	}
	if first > second {
		t.Fatalf("BuildSkeleton() headers out of order:\n%s", got)
	}
}

func TestBuildSkeletonDropsUnexportedSymbolsBeforeExportedOnBudgetOverflow(t *testing.T) {
	idx := samplePlannerFileIndex()

	got := BuildSkeleton([]string{"internal/auth/jwt.go"}, idx, 33)

	if !strings.Contains(got, "func GenerateToken() string  // internal/auth/jwt.go:10") {
		t.Fatalf("BuildSkeleton() dropped exported function:\n%s", got)
	}
	if !strings.Contains(got, "type Claims struct  // internal/auth/jwt.go:2") {
		t.Fatalf("BuildSkeleton() dropped exported type:\n%s", got)
	}
	if strings.Contains(got, "generateTokenSecret") {
		t.Fatalf("BuildSkeleton() kept unexported function despite budget overflow:\n%s", got)
	}
	if strings.Contains(got, "tokenConfig") {
		t.Fatalf("BuildSkeleton() kept unexported type despite budget overflow:\n%s", got)
	}
}

func TestBuildSkeletonStaysWithinTokenBudget(t *testing.T) {
	idx := samplePlannerFileIndex()

	got := BuildSkeleton([]string{"internal/auth/jwt.go"}, idx, 28)

	if estimateTokens(got) > 28 {
		t.Fatalf("estimateTokens(BuildSkeleton()) = %d, want <= 28\n%s", estimateTokens(got), got)
	}
}

func TestBuildFullSkeletonIncludesAllFilesWhenUnderBudget(t *testing.T) {
	idx := samplePlannerFileIndex()

	got := BuildFullSkeleton(idx, 10_000)

	if !strings.Contains(got, "// === internal/auth/jwt.go ===") {
		t.Fatalf("BuildFullSkeleton() missing jwt header:\n%s", got)
	}
	if !strings.Contains(got, "// === internal/auth/middleware.go ===") {
		t.Fatalf("BuildFullSkeleton() missing middleware header:\n%s", got)
	}
	if !strings.Contains(got, "func Middleware()  // internal/auth/middleware.go:5") {
		t.Fatalf("BuildFullSkeleton() missing middleware function:\n%s", got)
	}

	first := strings.Index(got, "// === internal/auth/jwt.go ===")
	second := strings.Index(got, "// === internal/auth/middleware.go ===")
	if first > second {
		t.Fatalf("BuildFullSkeleton() headers out of order:\n%s", got)
	}
}

func TestBuildFullSkeletonUsesSameExportedFirstTruncationRule(t *testing.T) {
	idx := samplePlannerFileIndex()

	got := BuildFullSkeleton(idx, 56)

	if strings.Contains(got, "generateTokenSecret") {
		t.Fatalf("BuildFullSkeleton() kept unexported function despite budget overflow:\n%s", got)
	}
	if strings.Contains(got, "tokenConfig") {
		t.Fatalf("BuildFullSkeleton() kept unexported type despite budget overflow:\n%s", got)
	}
	if !strings.Contains(got, "func Middleware()  // internal/auth/middleware.go:5") {
		t.Fatalf("BuildFullSkeleton() dropped exported middleware function:\n%s", got)
	}
	if estimateTokens(got) > 56 {
		t.Fatalf("estimateTokens(BuildFullSkeleton()) = %d, want <= 56\n%s", estimateTokens(got), got)
	}
}
