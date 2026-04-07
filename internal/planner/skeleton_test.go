package planner

import (
	"strings"
	"testing"

	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

func samplePlannerFileIndex() store.FileIndex {
	return store.FileIndex{
		"internal/auth/jwt.go": {
			Functions: []*store.FunctionDecl{{
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
			Types: []*store.TypeDecl{{
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
			Functions: []*store.FunctionDecl{{
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

func plannerFileIndex() store.FileIndex {
	return store.FileIndex{
		"internal/auth/jwt.go": {
			Functions: []*store.FunctionDecl{{
				Name:      "GenerateToken",
				Path:      "internal/auth/jwt.go",
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
		"internal/auth/middleware.go": {
			Functions: []*store.FunctionDecl{{
				Name:      "Middleware",
				Path:      "internal/auth/middleware.go",
				Signature: "func Middleware()",
				LineStart: 5,
				Exported:  true,
			}},
			Types: []*store.TypeDecl{{
				Name:      "Handler",
				Kind:      "interface",
				LineStart: 3,
				Exported:  true,
			}},
			Imports: []*store.Import{{
				Path:         "github.com/example/internal/auth",
				Internal:     true,
				ResolvedPath: "internal/auth/jwt.go",
			}},
		},
		"pkg/crypto/hash.go": {
			Types: []*store.TypeDecl{{
				Name:      "Hasher",
				Kind:      "interface",
				LineStart: 5,
				Exported:  true,
			}},
			Imports: []*store.Import{{
				Path:     "crypto/sha256",
				Internal: false,
			}},
		},
		"internal/util/empty.go": {},
	}
}

func TestBuildPlannerSkeletonOutputsFilePathTypesAndImports(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	// File path headers
	if !strings.Contains(got, "// internal/auth/jwt.go") {
		t.Fatalf("missing jwt.go file header:\n%s", got)
	}
	if !strings.Contains(got, "// internal/auth/middleware.go") {
		t.Fatalf("missing middleware.go file header:\n%s", got)
	}
	// Exported type names
	if !strings.Contains(got, "type Claims") {
		t.Fatalf("missing Claims type:\n%s", got)
	}
	if !strings.Contains(got, "type Handler") {
		t.Fatalf("missing Handler type:\n%s", got)
	}
	// Internal import relationships
	if !strings.Contains(got, "-> pkg/crypto/hash.go") {
		t.Fatalf("missing internal import for jwt.go:\n%s", got)
	}
	if !strings.Contains(got, "-> internal/auth/jwt.go") {
		t.Fatalf("missing internal import for middleware.go:\n%s", got)
	}
}

func TestBuildPlannerSkeletonExcludesFunctionSignatures(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	if strings.Contains(got, "GenerateToken") {
		t.Fatalf("should not contain function names, got:\n%s", got)
	}
	if strings.Contains(got, "Middleware") {
		t.Fatalf("should not contain function names, got:\n%s", got)
	}
	if strings.Contains(got, "func ") {
		t.Fatalf("should not contain any func keyword, got:\n%s", got)
	}
}

func TestBuildPlannerSkeletonTruncatesAtFileGranularity(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10)

	// Must stay within token budget
	if estimateTokens(got) > 10 {
		t.Fatalf("estimateTokens() = %d, want <= 10\n%s", estimateTokens(got), got)
	}

	// If a file appears, its content must be complete (no partial truncation)
	if strings.Contains(got, "// internal/auth/jwt.go") {
		if !strings.Contains(got, "type Claims") {
			t.Fatalf("jwt.go header present but type line missing (partial file):\n%s", got)
		}
		if !strings.Contains(got, "-> pkg/crypto/hash.go") {
			t.Fatalf("jwt.go header present but import line missing (partial file):\n%s", got)
		}
	}
}

func TestBuildPlannerSkeletonIncludesEmptyFilesAsPlaceholder(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	if !strings.Contains(got, "// internal/util/empty.go") {
		t.Fatalf("empty file should appear as placeholder:\n%s", got)
	}
	// Ensure no type or import lines after empty file header
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if line == "// internal/util/empty.go" {
			// Next line should not be indented (no type or import)
			if i+1 < len(lines) && (strings.HasPrefix(lines[i+1], "  type") || strings.HasPrefix(lines[i+1], "  ->")) {
				t.Fatalf("empty file should have no type/import lines:\n%s", got)
			}
			return
		}
	}
}

func TestBuildPlannerSkeletonIgnoresExternalImports(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	// pkg/crypto/hash.go has only external imports (crypto/sha256)
	// It should NOT show any -> line
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if line == "// pkg/crypto/hash.go" {
			if i+1 < len(lines) {
				next := lines[i+1]
				if strings.HasPrefix(next, "  ->") {
					t.Fatalf("external import should not appear in -> line:\n%s", got)
				}
			}
			return
		}
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

func TestBuildPlannerSkeletonWithImportanceNilFilter(t *testing.T) {
	t.Parallel()
	idx := plannerFileIndex()

	got := BuildPlannerSkeletonWithImportance(idx, 10_000, nil)

	// Should fallback to standard skeleton (no function names)
	if strings.Contains(got, "func ") {
		t.Fatalf("nil filter should use standard skeleton without functions, got:\n%s", got)
	}
}

func TestBuildPlannerSkeletonWithImportanceMarkers(t *testing.T) {
	idx := plannerFileIndex()

	// Create testMetrics where GenerateToken is important, Middleware is not
	testMetrics := store.MetricsMap{
		"internal/auth/jwt.go#GenerateToken":    {FuncID: "internal/auth/jwt.go#GenerateToken", ImportanceScore: 0.95},
		"internal/auth/middleware.go#Middleware": {FuncID: "internal/auth/middleware.go#Middleware", ImportanceScore: 0.3},
	}
	filter := metrics.NewImportanceFilter(testMetrics, 0)

	got := BuildPlannerSkeletonWithImportance(idx, 10_000, filter)

	// Important function should have ★ marker
	if !strings.Contains(got, "★") {
		t.Fatalf("expected ★ marker for important functions, got:\n%s", got)
	}
	// Important function signature should be present
	if !strings.Contains(got, "GenerateToken") {
		t.Fatalf("expected important function name in importance skeleton, got:\n%s", got)
	}
	// Non-important functions should be excluded to keep skeleton compact
	if strings.Contains(got, "Middleware") {
		t.Fatalf("non-important function Middleware should be excluded, got:\n%s", got)
	}
}

func TestBuildPlannerSkeletonWithImportanceOrdersFilesByScore(t *testing.T) {
	idx := store.FileIndex{
		"pkg/low.go": {
			Path: "pkg/low.go",
			Functions: []*store.FunctionDecl{{
				Name: "Helper", Path: "pkg/low.go", Signature: "func Helper()",
				LineStart: 1, Exported: false,
			}},
		},
		"pkg/high.go": {
			Path: "pkg/high.go",
			Functions: []*store.FunctionDecl{{
				Name: "Main", Path: "pkg/high.go", Signature: "func Main()",
				LineStart: 1, Exported: true,
			}},
		},
	}
	testMetrics := store.MetricsMap{
		"pkg/low.go#Helper": {FuncID: "pkg/low.go#Helper", ImportanceScore: 0.1},
		"pkg/high.go#Main":  {FuncID: "pkg/high.go#Main", ImportanceScore: 0.9},
	}
	filter := metrics.NewImportanceFilter(testMetrics, 0)

	got := BuildPlannerSkeletonWithImportance(idx, 10_000, filter)

	highIdx := strings.Index(got, "pkg/high.go")
	lowIdx := strings.Index(got, "pkg/low.go")
	if highIdx == -1 || lowIdx == -1 {
		t.Fatalf("missing file headers:\n%s", got)
	}
	if highIdx > lowIdx {
		t.Fatalf("high.go should appear before low.go (higher importance), got:\n%s", got)
	}
}

func TestBuildPlannerSkeletonWithImportanceRespectsTokenBudget(t *testing.T) {
	idx := plannerFileIndex()
	testMetrics := store.MetricsMap{
		"internal/auth/jwt.go#GenerateToken": {FuncID: "internal/auth/jwt.go#GenerateToken", ImportanceScore: 0.9},
	}
	filter := metrics.NewImportanceFilter(testMetrics, 0)

	got := BuildPlannerSkeletonWithImportance(idx, 15, filter)

	if estimateTokens(got) > 15 {
		t.Fatalf("estimateTokens() = %d, want <= 15\n%s", estimateTokens(got), got)
	}
}
