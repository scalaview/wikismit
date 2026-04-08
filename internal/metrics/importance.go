package metrics

import (
	"sort"

	"github.com/scalaview/wikismit/pkg/store"
)

// ImportanceFilter provides threshold-based filtering and ordering of functions
// by their importance scores. It consumes the MetricsMap produced by MetricsComputer.
type ImportanceFilter struct {
	metrics   store.MetricsMap
	threshold float64
	p75       float64 // precomputed 75th percentile for IsImportant
}

// NewImportanceFilter creates a filter with the given metrics and minimum score threshold.
// The 75th percentile is precomputed at construction time so IsImportant runs in O(1).
func NewImportanceFilter(metrics store.MetricsMap, threshold float64) *ImportanceFilter {
	scores := make([]float64, 0, len(metrics))
	for _, fm := range metrics {
		scores = append(scores, fm.ImportanceScore)
	}
	return &ImportanceFilter{
		metrics:   metrics,
		threshold: threshold,
		p75:       percentile(scores, 75),
	}
}

// ShouldSummarize returns true if the function's importance score meets the threshold.
// Functions without metrics are always included (conservative default).
func (f *ImportanceFilter) ShouldSummarize(funcID string) bool {
	m, ok := f.metrics[funcID]
	if !ok {
		return true
	}
	return m.ImportanceScore >= f.threshold
}

// Metrics returns the underlying MetricsMap. Exported so consumers in other
// packages (e.g. planner) can access score data without duplicating logic.
func (f *ImportanceFilter) Metrics() store.MetricsMap {
	return f.metrics
}

// SortByImportance sorts function IDs by importance score (descending).
// Functions without metrics are placed last, preserving their relative order.
func (f *ImportanceFilter) SortByImportance(funcIDs []string) []string {
	sorted := make([]string, len(funcIDs))
	copy(sorted, funcIDs)

	score := func(id string) float64 {
		if m, ok := f.metrics[id]; ok {
			return m.ImportanceScore
		}
		return -1
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		return score(sorted[i]) > score(sorted[j])
	})
	return sorted
}

// TopN returns the N most important function IDs from the candidates.
// If n > len(candidates), all candidates are returned.
func (f *ImportanceFilter) TopN(candidates []string, n int) []string {
	sorted := f.SortByImportance(candidates)
	if n >= len(sorted) {
		return sorted
	}
	return sorted[:n]
}

// IsImportant returns true if the function's score is above the 75th percentile
// of all known scores. This is used to mark "key functions" in planner skeletons.
// The percentile is precomputed at construction time, so this runs in O(1).
func (f *ImportanceFilter) IsImportant(funcID string) bool {
	m, ok := f.metrics[funcID]
	if !ok {
		return false
	}
	return m.ImportanceScore > f.p75
}

// percentile calculates the p-th percentile of a slice of float64 values
// using linear interpolation between closest ranks.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) == 1 {
		return values[0]
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(rank)
	if lower >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lower)
	return sorted[lower] + frac*(sorted[lower+1]-sorted[lower])
}
