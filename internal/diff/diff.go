package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
)

type Verdict string

const (
	VerdictImproved  Verdict = "IMPROVED"
	VerdictRegressed Verdict = "REGRESSED"
	VerdictMixed     Verdict = "MIXED"
	VerdictUnchanged Verdict = "UNCHANGED"
)

type FloatDelta struct {
	Before   float64 `json:"before"`
	After    float64 `json:"after"`
	Delta    float64 `json:"delta"`
	DeltaPct float64 `json:"delta_pct"`
}

type IntDelta struct {
	Before int `json:"before"`
	After  int `json:"after"`
	Delta  int `json:"delta"`
}

type Report struct {
	Verdict         Verdict    `json:"verdict"`
	Summary         string     `json:"summary"`
	TotalEvents     IntDelta   `json:"total_events"`
	PauseAvgMs      FloatDelta `json:"pause_avg_ms"`
	PauseP50Ms      FloatDelta `json:"pause_p50_ms"`
	PauseP95Ms      FloatDelta `json:"pause_p95_ms"`
	PauseP99Ms      FloatDelta `json:"pause_p99_ms"`
	PauseMaxMs      FloatDelta `json:"pause_max_ms"`
	From50To150     IntDelta   `json:"from_50_to_150ms"`
	Above150ms      IntDelta   `json:"above_150ms"`
	FullCount       IntDelta   `json:"full_gc_count"`
	GCOverheadPct   FloatDelta `json:"gc_overhead_pct"`
	AvgAllocRateMBs FloatDelta `json:"avg_alloc_rate_mbs"`
}

// Compare calculates metric deltas and an overall improvement verdict between two logs.
func Compare(before, after analyzer.Metrics) Report {
	rep := Report{
		TotalEvents:     IntDelta{Before: before.TotalEvents, After: after.TotalEvents, Delta: after.TotalEvents - before.TotalEvents},
		PauseAvgMs:      floatDelta(durMs(before.PauseAvg), durMs(after.PauseAvg)),
		PauseP50Ms:      floatDelta(durMs(before.PauseP50), durMs(after.PauseP50)),
		PauseP95Ms:      floatDelta(durMs(before.PauseP95), durMs(after.PauseP95)),
		PauseP99Ms:      floatDelta(durMs(before.PauseP99), durMs(after.PauseP99)),
		PauseMaxMs:      floatDelta(durMs(before.PauseMax), durMs(after.PauseMax)),
		From50To150:     IntDelta{Before: before.From50To150, After: after.From50To150, Delta: after.From50To150 - before.From50To150},
		Above150ms:      IntDelta{Before: before.Above150ms, After: after.Above150ms, Delta: after.Above150ms - before.Above150ms},
		FullCount:       IntDelta{Before: before.FullCount, After: after.FullCount, Delta: after.FullCount - before.FullCount},
		GCOverheadPct:   floatDelta(before.GCOverheadPct, after.GCOverheadPct),
		AvgAllocRateMBs: floatDelta(before.AvgAllocRateMBs, after.AvgAllocRateMBs),
	}

	improved := 0
	regressed := 0

	if rep.PauseP99Ms.DeltaPct <= -10 {
		improved++
	} else if rep.PauseP99Ms.DeltaPct >= 10 {
		regressed++
	}
	if rep.PauseMaxMs.DeltaPct <= -10 {
		improved++
	} else if rep.PauseMaxMs.DeltaPct >= 10 {
		regressed++
	}
	if rep.Above150ms.Delta < 0 {
		improved++
	} else if rep.Above150ms.Delta > 0 {
		regressed++
	}
	if rep.FullCount.Delta < 0 {
		improved++
	} else if rep.FullCount.Delta > 0 {
		regressed++
	}

	switch {
	case improved > 0 && regressed == 0:
		rep.Verdict = VerdictImproved
		rep.Summary = fmt.Sprintf("p99 pause changed by %+.1f%% (%.2fms -> %.2fms); severe freezes changed by %+d.",
			rep.PauseP99Ms.DeltaPct, rep.PauseP99Ms.Before, rep.PauseP99Ms.After, rep.Above150ms.Delta)
	case regressed > 0 && improved == 0:
		rep.Verdict = VerdictRegressed
		rep.Summary = fmt.Sprintf("GC pause tail regressed (p99 %+.1f%%, severe freezes %+d).",
			rep.PauseP99Ms.DeltaPct, rep.Above150ms.Delta)
	case improved > 0 && regressed > 0:
		rep.Verdict = VerdictMixed
		rep.Summary = fmt.Sprintf("Mixed impact: p99 changed by %+.1f%% while severe freezes changed by %+d.",
			rep.PauseP99Ms.DeltaPct, rep.Above150ms.Delta)
	default:
		rep.Verdict = VerdictUnchanged
		rep.Summary = "No significant change in GC pause latency or freeze counts."
	}

	return rep
}

func RenderText(w io.Writer, beforeLabel, afterLabel string, rep Report) {
	fmt.Fprintf(w, "=== TickDoctor A/B GC Comparison ===\n\n")
	fmt.Fprintf(w, "  Before: %s (%d events)\n", beforeLabel, rep.TotalEvents.Before)
	fmt.Fprintf(w, "  After:  %s (%d events)\n\n", afterLabel, rep.TotalEvents.After)
	fmt.Fprintf(w, "Verdict: [%s] %s\n\n", rep.Verdict, rep.Summary)

	fmt.Fprintf(w, "%-22s %-12s %-12s %-16s\n", "Metric", "Before", "After", "Delta")
	fmt.Fprintf(w, "%-22s %-12s %-12s %-16s\n", "------", "------", "-----", "-----")

	printMsRow(w, "Avg Pause", rep.PauseAvgMs)
	printMsRow(w, "p50 Pause", rep.PauseP50Ms)
	printMsRow(w, "p95 Pause", rep.PauseP95Ms)
	printMsRow(w, "p99 Pause", rep.PauseP99Ms)
	printMsRow(w, "Max Pause", rep.PauseMaxMs)

	fmt.Fprintf(w, "%-22s %-12d %-12d %+d\n", "Stutters (50-150ms)", rep.From50To150.Before, rep.From50To150.After, rep.From50To150.Delta)
	fmt.Fprintf(w, "%-22s %-12d %-12d %+d\n", "Severe (>150ms)", rep.Above150ms.Before, rep.Above150ms.After, rep.Above150ms.Delta)
	fmt.Fprintf(w, "%-22s %-12d %-12d %+d\n", "Full GC Count", rep.FullCount.Before, rep.FullCount.After, rep.FullCount.Delta)

	if rep.GCOverheadPct.Before > 0 || rep.GCOverheadPct.After > 0 {
		fmt.Fprintf(w, "%-22s %-12s %-12s %+.2f%%\n",
			"GC Overhead",
			fmt.Sprintf("%.2f%%", rep.GCOverheadPct.Before),
			fmt.Sprintf("%.2f%%", rep.GCOverheadPct.After),
			rep.GCOverheadPct.Delta,
		)
	}
	if rep.AvgAllocRateMBs.Before > 0 || rep.AvgAllocRateMBs.After > 0 {
		fmt.Fprintf(w, "%-22s %-12s %-12s %+.1f MB/s (%.1f%%)\n",
			"Alloc Rate",
			fmt.Sprintf("%.1f MB/s", rep.AvgAllocRateMBs.Before),
			fmt.Sprintf("%.1f MB/s", rep.AvgAllocRateMBs.After),
			rep.AvgAllocRateMBs.Delta,
			rep.AvgAllocRateMBs.DeltaPct,
		)
	}
}

func RenderJSON(w io.Writer, rep Report) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func printMsRow(w io.Writer, name string, d FloatDelta) {
	fmt.Fprintf(w, "%-22s %-12s %-12s %+.2fms (%.1f%%)\n",
		name,
		fmt.Sprintf("%.2fms", d.Before),
		fmt.Sprintf("%.2fms", d.After),
		d.Delta,
		d.DeltaPct,
	)
}

func durMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func floatDelta(before, after float64) FloatDelta {
	delta := after - before
	pct := 0.0
	if before > 0 {
		pct = math.Round((delta/before)*1000.0) / 10.0
	} else if after > 0 {
		pct = 100.0
	}
	return FloatDelta{
		Before:   before,
		After:    after,
		Delta:    delta,
		DeltaPct: pct,
	}
}
