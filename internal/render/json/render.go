package json

import (
	"encoding/json"
	"io"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
)

type JVMReport struct {
	Collector    string `json:"collector"`
	Version      string `json:"version,omitempty"`
	MaxHeapMB    int64  `json:"max_heap_mb,omitempty"`
	InitHeapMB   int64  `json:"init_heap_mb,omitempty"`
	RegionSizeMB int64  `json:"region_size_mb,omitempty"`
}

type MetricsReport struct {
	TotalEvents     int     `json:"total_events"`
	TotalPauseMs    float64 `json:"total_pause_ms"`
	AvgPauseMs      float64 `json:"avg_pause_ms"`
	MaxPauseMs      float64 `json:"max_pause_ms"`
	P50Ms           float64 `json:"p50_ms"`
	P90Ms           float64 `json:"p90_ms"`
	P95Ms           float64 `json:"p95_ms"`
	P99Ms           float64 `json:"p99_ms"`
	Under25ms       int     `json:"under_25ms"`
	From25To50ms    int     `json:"from_25_to_50ms"`
	From50To150ms   int     `json:"from_50_to_150ms"`
	Above150ms      int     `json:"above_150ms"`
	AvgAllocRateMBs float64 `json:"avg_alloc_rate_mbs,omitempty"`
	GCOverheadPct   float64 `json:"gc_overhead_pct,omitempty"`
}

type FindingReport struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Severity       string `json:"severity"`
	Evidence       string `json:"evidence,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	Explanation    string `json:"explanation,omitempty"`
	DocURL         string `json:"doc_url,omitempty"`
}

type CorrelationReport struct {
	Verdict             string  `json:"verdict"`
	VerdictTitle        string  `json:"verdict_title"`
	Summary             string  `json:"summary"`
	TotalLagEvents      int     `json:"total_lag_events"`
	CorrelatedLagEvents int     `json:"correlated_lag_events"`
	CorrelationRatio    float64 `json:"correlation_ratio"`
	TotalLagDurationMs  float64 `json:"total_lag_duration_ms"`
	ExplainedGCDurMs    float64 `json:"explained_gc_duration_ms"`
	ExplainedRatio      float64 `json:"explained_ratio"`
}

type FullReport struct {
	Timestamp   time.Time          `json:"timestamp"`
	JVM         JVMReport          `json:"jvm"`
	Metrics     MetricsReport      `json:"metrics"`
	Findings    []FindingReport    `json:"findings"`
	Correlation *CorrelationReport `json:"correlation,omitempty"`
}

// Render writes the diagnostic report formatted as clean JSON.
func Render(w io.Writer, jvm model.JVMInfo, m analyzer.Metrics, findings []rules.Finding, corr *correlate.Report) error {
	rep := FullReport{
		Timestamp: time.Now().UTC(),
		JVM: JVMReport{
			Collector:    string(jvm.Collector),
			Version:      jvm.FullVersion,
			MaxHeapMB:    jvm.HeapMax / (1024 * 1024),
			InitHeapMB:   jvm.HeapInitial / (1024 * 1024),
			RegionSizeMB: jvm.RegionSize / (1024 * 1024),
		},
		Metrics: MetricsReport{
			TotalEvents:     m.TotalEvents,
			TotalPauseMs:    float64(m.PauseSum.Microseconds()) / 1000.0,
			AvgPauseMs:      float64(m.PauseAvg.Microseconds()) / 1000.0,
			MaxPauseMs:      float64(m.PauseMax.Microseconds()) / 1000.0,
			P50Ms:           float64(m.PauseP50.Microseconds()) / 1000.0,
			P90Ms:           float64(m.PauseP90.Microseconds()) / 1000.0,
			P95Ms:           float64(m.PauseP95.Microseconds()) / 1000.0,
			P99Ms:           float64(m.PauseP99.Microseconds()) / 1000.0,
			Under25ms:       m.Under25ms,
			From25To50ms:    m.From25To50,
			From50To150ms:   m.From50To150,
			Above150ms:      m.Above150ms,
			AvgAllocRateMBs: m.AvgAllocRateMBs,
			GCOverheadPct:   m.GCOverheadPct,
		},
		Findings: make([]FindingReport, 0, len(findings)),
	}

	for _, f := range findings {
		rep.Findings = append(rep.Findings, FindingReport{
			ID:             f.Rule.ID,
			Title:          f.Rule.Title,
			Severity:       string(f.Rule.Severity),
			Evidence:       f.Evidence,
			Recommendation: f.Recommendation,
			Explanation:    f.Rule.Explanation,
			DocURL:         f.Rule.DocURL,
		})
	}

	if corr != nil && corr.Verdict != correlate.VerdictNoLag {
		rep.Correlation = &CorrelationReport{
			Verdict:             string(corr.Verdict),
			VerdictTitle:        corr.VerdictTitle,
			Summary:             corr.VerdictMessage,
			TotalLagEvents:      corr.TotalLagEvents,
			CorrelatedLagEvents: corr.CorrelatedLagEvents,
			CorrelationRatio:    corr.LagCorrelationRatio,
			TotalLagDurationMs:  float64(corr.TotalLagDuration.Microseconds()) / 1000.0,
			ExplainedGCDurMs:    float64(corr.ExplainedGCDuration.Microseconds()) / 1000.0,
			ExplainedRatio:      corr.ExplainedRatio,
		}
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
