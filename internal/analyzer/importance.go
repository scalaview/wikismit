package analyzer

import (
	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

// ImportanceFilter is an alias for metrics.ImportanceFilter for backward compatibility.
// Deprecated: Use metrics.ImportanceFilter directly.
type ImportanceFilter = metrics.ImportanceFilter

// NewImportanceFilter is an alias for metrics.NewImportanceFilter for backward compatibility.
// Deprecated: Use metrics.NewImportanceFilter directly.
func NewImportanceFilter(storeMetrics store.MetricsMap, threshold float64) *metrics.ImportanceFilter {
	return metrics.NewImportanceFilter(storeMetrics, threshold)
}

