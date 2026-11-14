package correlate

import (
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

func TestCorrelateGCPrimary(t *testing.T) {
	baseTime := time.Date(2026, 11, 15, 18, 0, 0, 0, time.UTC)

	gcEvents := []model.GCEvent{
		{
			ID:        1,
			StartTime: baseTime.Add(60 * time.Second),
			Duration:  1500 * time.Millisecond,
			Type:      model.GCTypeFull,
			Reason:    "System.gc()",
		},
		{
			ID:        2,
			StartTime: baseTime.Add(120 * time.Second),
			Duration:  800 * time.Millisecond,
			Type:      model.GCTypeYoung,
			Reason:    "G1 Evacuation Pause",
		},
	}

	lagEvents := []model.LagEvent{
		{
			Time:        baseTime.Add(60*time.Second + 1550*time.Millisecond),
			Duration:    1600 * time.Millisecond,
			TicksBehind: 32,
			Source:      "TickLoop",
		},
		{
			Time:        baseTime.Add(120*time.Second + 850*time.Millisecond),
			Duration:    850 * time.Millisecond,
			TicksBehind: 17,
			Source:      "TickLoop",
		},
	}

	rep := Correlate(gcEvents, lagEvents, Options{Tolerance: 1500 * time.Millisecond})

	if rep.Verdict != VerdictGCPrimary {
		t.Fatalf("expected VerdictGCPrimary, got %v", rep.Verdict)
	}
	if rep.TotalLagEvents != 2 {
		t.Errorf("expected 2 lag events, got %d", rep.TotalLagEvents)
	}
	if rep.CorrelatedLagEvents != 2 {
		t.Errorf("expected 2 correlated events, got %d", rep.CorrelatedLagEvents)
	}
	if rep.LagCorrelationRatio < 99.0 {
		t.Errorf("expected ~100%% correlation ratio, got %f", rep.LagCorrelationRatio)
	}
	if len(rep.Incidents) != 2 {
		t.Errorf("expected 2 incidents, got %d", len(rep.Incidents))
	}
}

func TestCorrelateGCHealthy(t *testing.T) {
	baseTime := time.Date(2026, 11, 15, 18, 0, 0, 0, time.UTC)

	gcEvents := []model.GCEvent{
		{
			ID:        1,
			StartTime: baseTime.Add(10 * time.Second),
			Duration:  12 * time.Millisecond,
			Type:      model.GCTypeYoung,
		},
	}

	lagEvents := []model.LagEvent{
		{
			Time:        baseTime.Add(300 * time.Second),
			Duration:    4500 * time.Millisecond,
			TicksBehind: 90,
			Source:      "TickLoop",
		},
		{
			Time:        baseTime.Add(400 * time.Second),
			Duration:    6000 * time.Millisecond,
			TicksBehind: 120,
			Source:      "TickLoop",
		},
	}

	rep := Correlate(gcEvents, lagEvents, Options{Tolerance: 1500 * time.Millisecond})

	if rep.Verdict != VerdictGCHealthy {
		t.Fatalf("expected VerdictGCHealthy, got %v", rep.Verdict)
	}
	if rep.CorrelatedLagEvents != 0 {
		t.Errorf("expected 0 correlated events, got %d", rep.CorrelatedLagEvents)
	}
	if rep.UncorrelatedLag != 2 {
		t.Errorf("expected 2 uncorrelated events, got %d", rep.UncorrelatedLag)
	}
}

func TestCorrelateNoLag(t *testing.T) {
	rep := Correlate(nil, nil, Options{})
	if rep.Verdict != VerdictNoLag {
		t.Fatalf("expected VerdictNoLag, got %v", rep.Verdict)
	}
}

func TestCorrelateMissingTimestamps(t *testing.T) {
	gcEvents := []model.GCEvent{
		{ID: 1, Uptime: 10 * time.Second, Duration: 250 * time.Millisecond},
	}
	lagEvents := []model.LagEvent{
		{Time: time.Now(), Duration: 300 * time.Millisecond, TicksBehind: 6},
	}
	rep := Correlate(gcEvents, lagEvents, Options{})
	if rep.Verdict != VerdictNoTimestamps {
		t.Fatalf("expected VerdictNoTimestamps, got %v", rep.Verdict)
	}
}
