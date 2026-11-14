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

	HumongousCount      int
	EvacuationFailures  int
	ExplicitGCCount     int
	MetaspaceGCCount    int
	CMSFailures         int
	ZGCAllocationStalls int

	AvgAllocRateMBs float64
	SuspectedLeak   bool

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

	if len(postGCHeaps) >= 10 {
		firstHalfAvg := average(postGCHeaps[:len(postGCHeaps)/2])
		secondHalfAvg := average(postGCHeaps[len(postGCHeaps)/2:])
		if float64(secondHalfAvg) > float64(firstHalfAvg)*1.4 && float64(m.MaxHeapAfter) > float64(m.HeapCapacity)*0.85 {
			m.SuspectedLeak = true
		}
	}

	span := lastUptime - firstUptime
	if span <= 0 && !firstTime.IsZero() && lastTime.After(firstTime) {
		span = lastTime.Sub(firstTime)
	}
	if span <= 0 && totalUptime > 0 {
		span = totalUptime
	}
	if span > 0 && allocBytes > 0 {
		m.AvgAllocRateMBs = float64(allocBytes) / (1024.0 * 1024.0) / span.Seconds()
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

func average(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return sum / int64(len(vals))
}
