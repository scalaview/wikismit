package metrics

import (
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

// buildTestMetrics creates a MetricsMap with varied importance scores for testing.
func buildTestMetrics() store.MetricsMap {
	return store.MetricsMap{
		"pkg/a.go#High":   {FuncID: "pkg/a.go#High", ImportanceScore: 0.9, IsEntryPoint: true},
		"pkg/a.go#Mid":    {FuncID: "pkg/a.go#Mid", ImportanceScore: 0.5},
		"pkg/a.go#Low":    {FuncID: "pkg/a.go#Low", ImportanceScore: 0.1},
		"pkg/b.go#Helper": {FuncID: "pkg/b.go#Helper", ImportanceScore: 0.05},
	}
}

func TestNewImportanceFilter(t *testing.T) {
	metrics := buildTestMetrics()
	threshold := 0.5

	f := NewImportanceFilter(metrics, threshold)

	if f.Metrics() == nil {
		t.Fatal("metrics should not be nil")
	}
	if len(f.Metrics()) != len(metrics) {
		t.Fatalf("metrics length = %d, want %d", len(f.Metrics()), len(metrics))
	}
}

func TestShouldSummarize(t *testing.T) {
	metrics := buildTestMetrics()
	f := NewImportanceFilter(metrics, 0.5)

	tests := []struct {
		name     string
		funcID   string
		want     bool
		reason   string
	}{
		{
			name:   "above threshold",
			funcID: "pkg/a.go#High",
			want:   true,
			reason: "score 0.9 >= 0.5",
		},
		{
			name:   "at threshold",
			funcID: "pkg/a.go#Mid",
			want:   true,
			reason: "score 0.5 >= 0.5",
		},
		{
			name:   "below threshold",
			funcID: "pkg/a.go#Low",
			want:   false,
			reason: "score 0.1 < 0.5",
		},
		{
			name:   "unknown function defaults to true",
			funcID: "unknown#Func",
			want:   true,
			reason: "conservative default for missing metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.ShouldSummarize(tt.funcID)
			if got != tt.want {
				t.Errorf("ShouldSummarize(%q) = %v, want %v (%s)", tt.funcID, got, tt.want, tt.reason)
			}
		})
	}
}

func TestSortByImportance(t *testing.T) {
	metrics := buildTestMetrics()
	f := NewImportanceFilter(metrics, 0)

	input := []string{"pkg/a.go#Low", "pkg/a.go#High", "pkg/b.go#Helper", "pkg/a.go#Mid"}
	got := f.SortByImportance(input)

	want := []string{"pkg/a.go#High", "pkg/a.go#Mid", "pkg/a.go#Low", "pkg/b.go#Helper"}
	if len(got) != len(want) {
		t.Fatalf("SortByImportance() length = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("SortByImportance()[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestTopN(t *testing.T) {
	metrics := buildTestMetrics()
	f := NewImportanceFilter(metrics, 0)

	input := []string{"pkg/a.go#Low", "pkg/a.go#High", "pkg/b.go#Helper", "pkg/a.go#Mid"}

	// N=2 should return top 2
	got := f.TopN(input, 2)
	if len(got) != 2 {
		t.Fatalf("TopN(..., 2) length = %d, want 2", len(got))
	}
	if got[0] != "pkg/a.go#High" {
		t.Errorf("TopN(..., 2)[0] = %s, want pkg/a.go#High", got[0])
	}
	if got[1] != "pkg/a.go#Mid" {
		t.Errorf("TopN(..., 2)[1] = %s, want pkg/a.go#Mid", got[1])
	}

	// N > len(input) should return all
	got = f.TopN(input, 100)
	if len(got) != len(input) {
		t.Fatalf("TopN(..., 100) length = %d, want %d", len(got), len(input))
	}
}

func TestIsImportant(t *testing.T) {
	tests := []struct {
		name     string
		metrics  store.MetricsMap
		funcID   string
		want     bool
		reason   string
	}{
		{
			name: "function above 75th percentile",
			metrics: store.MetricsMap{
				"pkg/a.go#High": {FuncID: "pkg/a.go#High", ImportanceScore: 0.9},
				"pkg/a.go#Mid":  {FuncID: "pkg/a.go#Mid", ImportanceScore: 0.5},
				"pkg/a.go#Low":  {FuncID: "pkg/a.go#Low", ImportanceScore: 0.1},
			},
			funcID: "pkg/a.go#High",
			want:   true,
			reason: "0.9 > 75th percentile (~0.65)",
		},
		{
			name: "function below 75th percentile",
			metrics: store.MetricsMap{
				"pkg/a.go#High": {FuncID: "pkg/a.go#High", ImportanceScore: 0.9},
				"pkg/a.go#Mid":  {FuncID: "pkg/a.go#Mid", ImportanceScore: 0.5},
				"pkg/a.go#Low":  {FuncID: "pkg/a.go#Low", ImportanceScore: 0.1},
			},
			funcID: "pkg/a.go#Mid",
			want:   false,
			reason: "0.5 <= 75th percentile (~0.65)",
		},
		{
			name: "unknown function",
			metrics: store.MetricsMap{
				"pkg/a.go#High": {FuncID: "pkg/a.go#High", ImportanceScore: 0.9},
			},
			funcID: "unknown#Func",
			want:   false,
			reason: "no metrics available",
		},
		{
			name:   "empty metrics",
			metrics: store.MetricsMap{},
			funcID: "pkg/a.go#High",
			want:   false,
			reason: "no metrics available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewImportanceFilter(tt.metrics, 0)
			got := f.IsImportant(tt.funcID)
			if got != tt.want {
				t.Errorf("IsImportant(%q) = %v, want %v (%s)", tt.funcID, got, tt.want, tt.reason)
			}
		})
	}
}

func TestNilMetricsMap(t *testing.T) {
	t.Parallel()
	f := NewImportanceFilter(nil, 0.5)

	t.Run("ShouldSummarize returns true for unknowns", func(t *testing.T) {
		if !f.ShouldSummarize("unknown#Func") {
			t.Fatal("ShouldSummarize on nil metrics should return true")
		}
	})

	t.Run("SortByImportance preserves input", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		got := f.SortByImportance(input)
		if len(got) != 3 {
			t.Fatalf("SortByImportance with nil metrics: got %d items, want 3", len(got))
		}
	})

	t.Run("TopN returns candidates unchanged", func(t *testing.T) {
		input := []string{"a", "b"}
		got := f.TopN(input, 5)
		if len(got) != 2 {
			t.Fatalf("TopN with nil metrics: got %d items, want 2", len(got))
		}
	})

	t.Run("IsImportant returns false", func(t *testing.T) {
		if f.IsImportant("any#Func") {
			t.Fatal("IsImportant on nil metrics should return false")
		}
	})

	t.Run("Metrics returns nil", func(t *testing.T) {
		if f.Metrics() != nil {
			t.Fatal("Metrics() should return nil")
		}
	})
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		p      float64
		want   float64
	}{
		{
			name:   "empty slice returns 0",
			values: []float64{},
			p:      50,
			want:   0,
		},
		{
			name:   "single value returns that value",
			values: []float64{0.5},
			p:      50,
			want:   0.5,
		},
		{
			name:   "median of two values averages them",
			values: []float64{0.0, 1.0},
			p:      50,
			want:   0.5,
		},
		{
			name:   "75th percentile of four values",
			values: []float64{0.1, 0.3, 0.5, 0.9},
			p:      75,
			want:   0.6, // rank = 0.75 * 3 = 2.25, so 0.5 + 0.25*(0.9-0.5) = 0.6
		},
		{
			name:   "25th percentile",
			values: []float64{0.1, 0.3, 0.5, 0.9},
			p:      25,
			want:   0.25, // Interpolated between 0.1 (index 0) and 0.3 (index 1)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.values, tt.p)
			if got != tt.want {
				t.Errorf("percentile(%v, %f) = %f, want %f", tt.values, tt.p, got, tt.want)
			}
		})
	}
}
