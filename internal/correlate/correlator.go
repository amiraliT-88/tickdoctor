package correlate

import (
	"fmt"
	"math"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

type Verdict string

const (
	VerdictGCPrimary    Verdict = "GC_PRIMARY"
	VerdictGCPartial    Verdict = "GC_PARTIAL"
	VerdictGCHealthy    Verdict = "GC_HEALTHY"
	VerdictNoLag        Verdict = "NO_LAG"
	VerdictNoTimestamps Verdict = "NO_TIMESTAMPS"
)

type Incident struct {
	LagEvent       model.LagEvent
	Pauses         []model.GCEvent
	GCPauseTotal   time.Duration
	ExplainedRatio float64 // GCPauseTotal / LagEvent.Duration * 100
}

type Options struct {
	Tolerance      time.Duration
	ServerStartRef time.Time
}

type Report struct {
	TotalLagEvents      int
	CorrelatedLagEvents int
	LagCorrelationRatio float64 // % of lag events coinciding with GC
	TotalLagDuration    time.Duration
	ExplainedGCDuration time.Duration
	ExplainedRatio      float64 // ExplainedGCDuration / TotalLagDuration * 100
	Verdict             Verdict
	VerdictTitle        string
	VerdictMessage      string
	Incidents           []Incident
	UncorrelatedLag     int
}

// Correlate aligns Minecraft lag events with JVM GC stop-the-world pauses.
func Correlate(gcEvents []model.GCEvent, lagEvents []model.LagEvent, opts Options) *Report {
	tolerance := opts.Tolerance
	if tolerance <= 0 {
		tolerance = 1500 * time.Millisecond
	}

	report := &Report{
		TotalLagEvents: len(lagEvents),
	}

	if len(lagEvents) == 0 {
		report.Verdict = VerdictNoLag
		report.VerdictTitle = "No Server Lag Detected"
		report.VerdictMessage = "No tick drops or watchdog freezes were detected in the server log."
		return report
	}

	timestampedGC := 0
	for _, gc := range gcEvents {
		if !gc.StartTime.IsZero() || (!opts.ServerStartRef.IsZero() && gc.Uptime > 0) {
			timestampedGC++
		}
	}
	if len(gcEvents) > 0 && timestampedGC == 0 {
		for _, lag := range lagEvents {
			report.TotalLagDuration += lag.Duration
		}
		report.UncorrelatedLag = len(lagEvents)
		report.Verdict = VerdictNoTimestamps
		report.VerdictTitle = "Cannot Correlate (Missing GC Timestamps)"
		report.VerdictMessage = "The GC log lacks wall-clock timestamps (:time or -XX:+PrintGCDateStamps), so pauses cannot be aligned with latest.log. Run 'tickdoctor setup' to enable timestamps."
		return report
	}

	for _, lag := range lagEvents {
		report.TotalLagDuration += lag.Duration

		lagStart := lag.Time.Add(-lag.Duration).Add(-tolerance)
		lagEnd := lag.Time.Add(tolerance)

		var matchedPauses []model.GCEvent
		var gcSum time.Duration

		for _, gc := range gcEvents {
			gcStart := gc.StartTime
			if gcStart.IsZero() && !opts.ServerStartRef.IsZero() && gc.Uptime > 0 {
				gcStart = opts.ServerStartRef.Add(gc.Uptime)
			}
			if gcStart.IsZero() {
				continue
			}

			gcEnd := gcStart.Add(gc.Duration)

			if gcStart.Before(lagEnd) && gcEnd.After(lagStart) {
				matchedPauses = append(matchedPauses, gc)
				gcSum += gc.Duration
			}
		}

		if len(matchedPauses) > 0 {
			report.CorrelatedLagEvents++
			report.ExplainedGCDuration += gcSum

			expRatio := 0.0
			if lag.Duration > 0 {
				expRatio = math.Min(100.0, float64(gcSum)/float64(lag.Duration)*100.0)
			}

			report.Incidents = append(report.Incidents, Incident{
				LagEvent:       lag,
				Pauses:         matchedPauses,
				GCPauseTotal:   gcSum,
				ExplainedRatio: expRatio,
			})
		} else {
			report.UncorrelatedLag++
		}
	}

	if report.TotalLagEvents > 0 {
		report.LagCorrelationRatio = float64(report.CorrelatedLagEvents) / float64(report.TotalLagEvents) * 100.0
	}
	if report.TotalLagDuration > 0 {
		report.ExplainedRatio = math.Min(100.0, float64(report.ExplainedGCDuration)/float64(report.TotalLagDuration)*100.0)
	}

	switch {
	case report.LagCorrelationRatio >= 50.0 || report.ExplainedRatio >= 50.0:
		report.Verdict = VerdictGCPrimary
		report.VerdictTitle = "GC is Primary Cause of Server Lag"
		report.VerdictMessage = fmt.Sprintf(
			"%.1f%% of lag spikes coincide directly with JVM GC pauses (%.1f%% of lag duration explained). Optimizing JVM flags and heap allocation will eliminate these tick drops.",
			report.LagCorrelationRatio,
			report.ExplainedRatio,
		)
	case report.LagCorrelationRatio >= 20.0 || report.ExplainedRatio >= 20.0:
		report.Verdict = VerdictGCPartial
		report.VerdictTitle = "GC is Contributing to Server Lag"
		report.VerdictMessage = fmt.Sprintf(
			"%.1f%% of lag spikes coincide with JVM GC pauses (%.1f%% of lag duration explained). Both JVM flag tuning and plugin profiling are recommended.",
			report.LagCorrelationRatio,
			report.ExplainedRatio,
		)
	default:
		report.Verdict = VerdictGCHealthy
		report.VerdictTitle = "GC is Healthy (Main-Thread Lag Detected)"
		report.VerdictMessage = "GC Stop-The-World pauses do not explain the server lag spikes. Lag is originating on the Minecraft main server thread (heavy plugins, entity count, chunk generation, or synchronous I/O). Profile your server with spark (/spark sampler)."
	}

	return report
}
