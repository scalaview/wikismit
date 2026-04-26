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

	if !strings.Contains(got, "FILE: internal/auth/jwt.go") {
		t.Fatalf("missing jwt.go FILE header:\n%s", got)
	}
	if !strings.Contains(got, "FILE: internal/auth/middleware.go") {
		t.Fatalf("missing middleware.go FILE header:\n%s", got)
	}
	if !strings.Contains(got, "TYPES") {
		t.Fatalf("missing TYPES section label:\n%s", got)
	}
	if !strings.Contains(got, "INTERNAL_IMPORTS") {
		t.Fatalf("missing INTERNAL_IMPORTS section label:\n%s", got)
	}
	if !strings.Contains(got, "  Claims struct") {
		t.Fatalf("missing Claims type:\n%s", got)
	}
	if !strings.Contains(got, "  Handler interface") {
		t.Fatalf("missing Handler type:\n%s", got)
	}
	// Internal import relationships
	if !strings.Contains(got, "  Claims struct") {
		t.Fatalf("missing Claims type:\n%s", got)
	}
	if !strings.Contains(got, "  Handler interface") {
		t.Fatalf("missing Handler type:\n%s", got)
	}
	// Internal import relationships
	if !strings.Contains(got, "  pkg/crypto/hash.go") {
		t.Fatalf("missing internal import for jwt.go:\n%s", got)
	}
	if !strings.Contains(got, "  internal/auth/jwt.go") {
		t.Fatalf("missing internal import for middleware.go:\n%s", got)
	}
}

func TestBuildPlannerSkeletonExcludesFunctionSignatures(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	if !strings.Contains(got, "QUERYABLE_FUNCTIONS") {
		t.Fatalf("missing QUERYABLE_FUNCTIONS section:\n%s", got)
	}
	if !strings.Contains(got, "internal/auth/jwt.go#GenerateToken") {
		t.Fatalf("missing function ID in QUERYABLE_FUNCTIONS:\n%s", got)
	}
	if !strings.Contains(got, "internal/auth/middleware.go#Middleware") {
		t.Fatalf("missing function ID in QUERYABLE_FUNCTIONS:\n%s", got)
	}
	if strings.Contains(got, "func GenerateToken") {
		t.Fatalf("should not contain function signature, got:\n%s", got)
	}
	if strings.Contains(got, "func Middleware") {
		t.Fatalf("should not contain function signature, got:\n%s", got)
	}
	if strings.Contains(got, "★") {
		t.Fatalf("should not contain star markers, got:\n%s", got)
	}
}

func TestBuildPlannerSkeletonTruncatesAtFileGranularity(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10)

	if estimateTokens(got) > 10 {
		t.Fatalf("estimateTokens() = %d, want <= 10\n%s", estimateTokens(got), got)
	}

	if strings.Contains(got, "FILE: internal/auth/jwt.go") {
		if !strings.Contains(got, "TYPES") {
			t.Fatalf("jwt.go header present but TYPES section missing (partial file):\n%s", got)
		}
		if !strings.Contains(got, "  Claims struct") {
			t.Fatalf("jwt.go header present but type line missing (partial file):\n%s", got)
		}
		if !strings.Contains(got, "INTERNAL_IMPORTS") {
			t.Fatalf("jwt.go header present but INTERNAL_IMPORTS section missing (partial file):\n%s", got)
		}
		if !strings.Contains(got, "  pkg/crypto/hash.go") {
			t.Fatalf("jwt.go header present but import line missing (partial file):\n%s", got)
		}
	}
}

func TestBuildPlannerSkeletonIncludesEmptyFilesAsPlaceholder(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	if !strings.Contains(got, "FILE: internal/util/empty.go") {
		t.Fatalf("empty file should appear as placeholder:\n%s", got)
	}
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if line == "FILE: internal/util/empty.go" {
			if i+1 < len(lines) {
				nextLine := lines[i+1]
				if nextLine == "QUERYABLE_FUNCTIONS" || nextLine == "TYPES" || nextLine == "INTERNAL_IMPORTS" {
					t.Fatalf("empty file should not have section labels:\n%s", got)
				}
				if strings.HasPrefix(nextLine, "  ") {
					t.Fatalf("empty file should have no indented content:\n%s", got)
				}
			}
			return
		}
	}
}

func TestBuildPlannerSkeletonIgnoresExternalImports(t *testing.T) {
	idx := plannerFileIndex()

	got := BuildPlannerSkeleton(idx, 10_000)

	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if line == "FILE: pkg/crypto/hash.go" {
			if i+1 < len(lines) {
				next := lines[i+1]
				if next == "INTERNAL_IMPORTS" {
					t.Fatalf("file with only external imports should not have INTERNAL_IMPORTS section:\n%s", got)
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
	standard := BuildPlannerSkeleton(idx, 10_000)

	if !strings.Contains(got, "QUERYABLE_FUNCTIONS") {
		t.Fatalf("nil filter should include QUERYABLE_FUNCTIONS section, got:\n%s", got)
	}
	if !strings.Contains(got, "internal/auth/jwt.go#GenerateToken") {
		t.Fatalf("nil filter should include function IDs, got:\n%s", got)
	}
	if !strings.Contains(standard, "QUERYABLE_FUNCTIONS") {
		t.Fatalf("standard skeleton missing QUERYABLE_FUNCTIONS:\n%s", standard)
	}
}

func TestBuildPlannerSkeletonWithImportanceIncludesQueryables(t *testing.T) {
	idx := plannerFileIndex()

	testMetrics := store.MetricsMap{
		"internal/auth/jwt.go#GenerateToken":    {FuncID: "internal/auth/jwt.go#GenerateToken", ImportanceScore: 0.95},
		"internal/auth/middleware.go#Middleware": {FuncID: "internal/auth/middleware.go#Middleware", ImportanceScore: 0.3},
	}
	filter := metrics.NewImportanceFilter(testMetrics, 0)

	got := BuildPlannerSkeletonWithImportance(idx, 10_000, filter)

	if !strings.Contains(got, "QUERYABLE_FUNCTIONS") {
		t.Fatalf("missing QUERYABLE_FUNCTIONS section:\n%s", got)
	}
	if !strings.Contains(got, "internal/auth/jwt.go#GenerateToken") {
		t.Fatalf("missing function ID in QUERYABLE_FUNCTIONS:\n%s", got)
	}
	if !strings.Contains(got, "internal/auth/middleware.go#Middleware") {
		t.Fatalf("missing function ID in QUERYABLE_FUNCTIONS:\n%s", got)
	}
	if strings.Contains(got, "★") {
		t.Fatalf("should not contain star markers, got:\n%s", got)
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

	highIdx := strings.Index(got, "FILE: pkg/high.go")
	lowIdx := strings.Index(got, "FILE: pkg/low.go")
	if highIdx == -1 || lowIdx == -1 {
		t.Fatalf("missing FILE headers:\n%s", got)
	}
	if highIdx > lowIdx {
		t.Fatalf("high.go should appear before low.go (higher importance), got:\n%s", got)
	}
	if !strings.Contains(got, "QUERYABLE_FUNCTIONS") {
		t.Fatalf("missing QUERYABLE_FUNCTIONS section:\n%s", got)
	}
	if !strings.Contains(got, "pkg/high.go#Main") {
		t.Fatalf("missing function ID:\n%s", got)
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
	if strings.Contains(got, "FILE: internal/auth/jwt.go") {
		if !strings.Contains(got, "QUERYABLE_FUNCTIONS") {
			t.Fatalf("importance skeleton included partial jwt.go block without QUERYABLE_FUNCTIONS:\n%s", got)
		}
		if !strings.Contains(got, "  internal/auth/jwt.go#GenerateToken") {
			t.Fatalf("importance skeleton included partial jwt.go block without function ref:\n%s", got)
		}
	}
}
