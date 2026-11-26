package analyzer

import (
	"sort"
	"strings"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

type Metrics struct {
	TotalEvents int

	PauseP50 time.Duration
	PauseP90 time.Duration
	PauseP95 time.Duration
	PauseP99 time.Duration
	PauseMax time.Duration
	PauseAvg time.Duration
	PauseSum time.Duration

	Under25ms  int
	From25To50 int
	From50To150 int
	Above150ms int

	YoungCount int
	MixedCount int
	FullCount  int

	HumongousCount        int
	EvacuationFailures    int
	ExplicitGCCount       int
	MetaspaceGCCount      int
	CMSFailures           int
	ZGCAllocationStalls   int
	ShenandoahDegenerated int

	AvgAllocRateMBs   float64
	GCOverheadPct     float64
	PostGCHeapSlopeMB float64
	SuspectedLeak     bool

	MaxHeapBefore int64
	MaxHeapAfter  int64
	HeapCapacity  int64
}

func Analyze(events []*model.GCEvent, totalUptime time.Duration) Metrics {
	var m Metrics
	m.TotalEvents = len(events)
	if len(events) == 0 {
		return m
	}

	pauses := make([]time.Duration, 0, len(events))
	var postGCHeaps []int64
	var allocBytes int64
	var prevHeapAfter int64
	var hasPrevHeap bool
	var firstUptime, lastUptime time.Duration
	var firstTime, lastTime time.Time

	for _, ev := range events {
		if ev.Uptime > 0 {
			if firstUptime == 0 || ev.Uptime < firstUptime {
				firstUptime = ev.Uptime
			}
			if ev.Uptime > lastUptime {
				lastUptime = ev.Uptime
			}
		}
		if !ev.StartTime.IsZero() {
			if firstTime.IsZero() || ev.StartTime.Before(firstTime) {
				firstTime = ev.StartTime
			}
			if lastTime.IsZero() || ev.StartTime.After(lastTime) {
				lastTime = ev.StartTime
			}
		}
		if hasPrevHeap && ev.HeapBefore > prevHeapAfter {
			allocBytes += ev.HeapBefore - prevHeapAfter
		}
		if ev.HeapAfter > 0 {
			prevHeapAfter = ev.HeapAfter
			hasPrevHeap = true
		} else {
			hasPrevHeap = false
		}

		pauses = append(pauses, ev.Duration)
		m.PauseSum += ev.Duration

		if ev.Duration < 25*time.Millisecond {
			m.Under25ms++
		} else if ev.Duration <= 50*time.Millisecond {
			m.From25To50++
		} else if ev.Duration <= 150*time.Millisecond {
			m.From50To150++
		} else {
			m.Above150ms++
		}

		switch ev.Type {
		case model.GCTypeYoung:
			m.YoungCount++
		case model.GCTypeMixed:
			m.MixedCount++
		case model.GCTypeFull:
			m.FullCount++
		}

		if ev.HumongousCount > 0 {
			m.HumongousCount += ev.HumongousCount
		}

		lowerReason := strings.ToLower(ev.Reason)
		if strings.Contains(lowerReason, "to-space exhausted") || strings.Contains(lowerReason, "evacuation failure") {
			m.EvacuationFailures++
		}
		if strings.Contains(lowerReason, "system.gc()") {
			m.ExplicitGCCount++
		}
		if strings.Contains(lowerReason, "metadata gc threshold") {
			m.MetaspaceGCCount++
		}
		if strings.Contains(lowerReason, "concurrent mode failure") {
			m.CMSFailures++
		}
		if strings.Contains(lowerReason, "allocation stall") {
			m.ZGCAllocationStalls++
		}
		if strings.Contains(lowerReason, "degenerated") {
			m.ShenandoahDegenerated++
		}

		if ev.HeapBefore > m.MaxHeapBefore {
			m.MaxHeapBefore = ev.HeapBefore
		}
		if ev.HeapAfter > m.MaxHeapAfter {
			m.MaxHeapAfter = ev.HeapAfter
		}
		if ev.HeapTotal > m.HeapCapacity {
			m.HeapCapacity = ev.HeapTotal
		}

		if ev.HeapAfter > 0 {
			postGCHeaps = append(postGCHeaps, ev.HeapAfter)
		}
	}

	sort.Slice(pauses, func(i, j int) bool {
		return pauses[i] < pauses[j]
	})

	m.PauseAvg = m.PauseSum / time.Duration(len(pauses))
	m.PauseMax = pauses[len(pauses)-1]
	m.PauseP50 = percentile(pauses, 0.50)
	m.PauseP90 = percentile(pauses, 0.90)
	m.PauseP95 = percentile(pauses, 0.95)
	m.PauseP99 = percentile(pauses, 0.99)

	m.SuspectedLeak, m.PostGCHeapSlopeMB = detectMemoryLeak(postGCHeaps, m.HeapCapacity)

	span := lastUptime - firstUptime
	if span <= 0 && !firstTime.IsZero() && lastTime.After(firstTime) {
		span = lastTime.Sub(firstTime)
	}
	if span <= 0 && totalUptime > 0 {
		span = totalUptime
	}
	if span > 0 {
		if allocBytes > 0 {
			m.AvgAllocRateMBs = float64(allocBytes) / (1024.0 * 1024.0) / span.Seconds()
		}
		if m.PauseSum > 0 {
			overhead := (float64(m.PauseSum) / float64(span)) * 100.0
			if overhead > 100.0 {
				overhead = 100.0
			}
			m.GCOverheadPct = overhead
		}
	}

	return m
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 || p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[n-1]
	}
	pos := p * float64(n-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	frac := pos - float64(lo)
	return sorted[lo] + time.Duration(frac*float64(sorted[hi]-sorted[lo]))
}

func detectMemoryLeak(postGCHeaps []int64, heapCap int64) (bool, float64) {
	if len(postGCHeaps) < 10 || heapCap <= 0 {
		return false, 0
	}

	start := len(postGCHeaps) / 5
	window := postGCHeaps[start:]
	n := len(window)
	if n < 8 {
		return false, 0
	}

	const mb = 1024.0 * 1024.0
	ys := make([]float64, n)
	var sumY, maxAfter float64
	for i, v := range window {
		y := float64(v) / mb
		ys[i] = y
		sumY += y
		if y > maxAfter {
			maxAfter = y
		}
	}

	meanX := float64(n-1) / 2.0
	meanY := sumY / float64(n)
	var ssXX, ssXY, ssYY float64
	for i, y := range ys {
		dx := float64(i) - meanX
		dy := y - meanY
		ssXX += dx * dx
		ssXY += dx * dy
		ssYY += dy * dy
	}
	if ssXX == 0 || ssYY == 0 {
		return false, 0
	}

	slope := ssXY / ssXX
	r2 := (ssXY * ssXY) / (ssXX * ssYY)
	capMB := float64(heapCap) / mb
	fittedGrowth := slope * float64(n-1)
	fittedStart := meanY - slope*meanX
	fittedEnd := fittedStart + fittedGrowth

	if slope > 0 && r2 >= 0.50 && maxAfter > capMB*0.75 &&
		(fittedGrowth >= capMB*0.25 || (fittedStart > 0 && fittedEnd > fittedStart*1.35)) {
		return true, slope
	}
	return false, slope
}
