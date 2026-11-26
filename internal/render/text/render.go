package text

import (
	"fmt"
	"io"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
)

func Render(w io.Writer, jvm model.JVMInfo, m analyzer.Metrics, findings []rules.Finding) {
	fmt.Fprintf(w, "=== TickDoctor GC Diagnostic Report ===\n\n")

	fmt.Fprintf(w, "JVM Configuration:\n")
	if jvm.Collector != model.CollectorUnknown {
		fmt.Fprintf(w, "  Collector: %s\n", jvm.Collector)
	}
	if jvm.FullVersion != "" {
		fmt.Fprintf(w, "  Version:   %s\n", jvm.FullVersion)
	}
	if jvm.HeapMax > 0 {
		fmt.Fprintf(w, "  Max Heap:  %d MB\n", jvm.HeapMax/(1024*1024))
	}
	if jvm.HeapInitial > 0 {
		fmt.Fprintf(w, "  Init Heap: %d MB\n", jvm.HeapInitial/(1024*1024))
	}
	if jvm.RegionSize > 0 {
		fmt.Fprintf(w, "  G1 Region: %d MB\n", jvm.RegionSize/(1024*1024))
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Pause Metrics (Total Events: %d, Total Time: %v):\n", m.TotalEvents, formatDuration(m.PauseSum))
	fmt.Fprintf(w, "  avg: %-9v  max: %-9v\n", formatDuration(m.PauseAvg), formatDuration(m.PauseMax))
	fmt.Fprintf(w, "  p50: %-9v  p90: %-9v  p95: %-9v  p99: %-9v\n",
		formatDuration(m.PauseP50), formatDuration(m.PauseP90),
		formatDuration(m.PauseP95), formatDuration(m.PauseP99))
	if m.AvgAllocRateMBs > 0 {
		fmt.Fprintf(w, "  alloc rate: %.1f MB/s", m.AvgAllocRateMBs)
		if m.GCOverheadPct > 0 {
			fmt.Fprintf(w, "  |  gc overhead: %.2f%% (throughput: %.2f%%)", m.GCOverheadPct, 100.0-m.GCOverheadPct)
		}
		fmt.Fprintln(w)
	} else if m.GCOverheadPct > 0 {
		fmt.Fprintf(w, "  gc overhead: %.2f%% (throughput: %.2f%%)\n", m.GCOverheadPct, 100.0-m.GCOverheadPct)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Tick Impact Distribution:\n")
	fmt.Fprintf(w, "  < 25ms   (healthy):               %-4d\n", m.Under25ms)
	fmt.Fprintf(w, "  25-50ms  (borderline 1 tick):     %-4d\n", m.From25To50)
	fmt.Fprintf(w, "  50-150ms (stutter, 1-3 ticks):    %-4d\n", m.From50To150)
	fmt.Fprintf(w, "  > 150ms  (severe freeze):         %-4d\n", m.Above150ms)
	fmt.Fprintln(w)

	if len(findings) == 0 {
		fmt.Fprintf(w, "Findings:\n")
		fmt.Fprintf(w, "  [HEALTHY] No GC configuration anomalies detected.\n")
		fmt.Fprintf(w, "  If your server experiences persistent low TPS, profile using spark (/spark sampler).\n")
		return
	}

	fmt.Fprintf(w, "Diagnoses & Recommended Fixes (%d):\n", len(findings))
	var suggestedFlags []string

	for _, f := range findings {
		fmt.Fprintf(w, "  [%s] %s (%s)\n", f.Rule.Severity, f.Rule.Title, f.Rule.ID)
		if f.Evidence != "" {
			fmt.Fprintf(w, "    Evidence:       %s\n", f.Evidence)
		}
		if f.Recommendation != "" {
			fmt.Fprintf(w, "    Recommendation: %s\n", f.Recommendation)
			suggestedFlags = append(suggestedFlags, f.Recommendation)
		}
		if f.Rule.Explanation != "" {
			fmt.Fprintf(w, "    Why:            %s\n", f.Rule.Explanation)
		}
		fmt.Fprintln(w)
	}

	if len(suggestedFlags) > 0 {
		fmt.Fprintf(w, "Suggested JVM Flag Action Plan:\n")
		for _, sf := range suggestedFlags {
			fmt.Fprintf(w, "  * %s\n", sf)
		}
		fmt.Fprintln(w)
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func RenderCorrelation(w io.Writer, corr *correlate.Report) {
	if corr == nil {
		return
	}
	fmt.Fprintf(w, "=== Server Lag Correlation (latest.log) ===\n\n")
	fmt.Fprintf(w, "Verdict: [%s] %s\n", corr.Verdict, corr.VerdictTitle)
	fmt.Fprintf(w, "Summary: %s\n\n", corr.VerdictMessage)
	fmt.Fprintf(w, "Statistics:\n")
	fmt.Fprintf(w, "  Total Lag Events:       %d (%v total lag duration)\n", corr.TotalLagEvents, formatDuration(corr.TotalLagDuration))
	fmt.Fprintf(w, "  Correlated with GC:     %d (%.1f%% of incidents)\n", corr.CorrelatedLagEvents, corr.LagCorrelationRatio)
	fmt.Fprintf(w, "  GC Freeze Time Share:   %v (%.1f%% of total lag explained)\n\n", formatDuration(corr.ExplainedGCDuration), corr.ExplainedRatio)

	if len(corr.Incidents) > 0 {
		fmt.Fprintf(w, "Lag Incidents Correlated with GC (showing up to 5):\n")
		limit := len(corr.Incidents)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			inc := corr.Incidents[i]
			fmt.Fprintf(w, "  * [%s] Server behind %v (%d ticks)\n",
				inc.LagEvent.Time.Format("15:04:05"),
				formatDuration(inc.LagEvent.Duration),
				inc.LagEvent.TicksBehind,
			)
			for _, p := range inc.Pauses {
				fmt.Fprintf(w, "    └─ Coincided with GC Pause: %v (%s: %s)\n",
					formatDuration(p.Duration),
					p.Type,
					p.Reason,
				)
			}
		}
		fmt.Fprintln(w)
	}
}

