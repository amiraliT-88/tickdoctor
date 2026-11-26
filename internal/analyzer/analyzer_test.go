package analyzer

import (
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

func TestAnalyzeMetrics(t *testing.T) {
	events := []*model.GCEvent{
		{ID: 1, Type: model.GCTypeYoung, Duration: 10 * time.Millisecond, HeapBefore: 100, HeapAfter: 40, HeapTotal: 1000},
		{ID: 2, Type: model.GCTypeYoung, Duration: 20 * time.Millisecond, HeapBefore: 200, HeapAfter: 50, HeapTotal: 1000},
		{ID: 3, Type: model.GCTypeYoung, Duration: 35 * time.Millisecond, HeapBefore: 300, HeapAfter: 60, HeapTotal: 1000},
		{ID: 4, Type: model.GCTypeYoung, Duration: 80 * time.Millisecond, HeapBefore: 400, HeapAfter: 70, HeapTotal: 1000, HumongousCount: 5},
		{ID: 5, Type: model.GCTypeFull, Duration: 1200 * time.Millisecond, Reason: "System.gc()", HeapBefore: 800, HeapAfter: 100, HeapTotal: 1000},
	}

	m := Analyze(events, 60*time.Second)

	if m.TotalEvents != 5 {
		t.Fatalf("expected 5 events, got %d", m.TotalEvents)
	}
	if m.YoungCount != 4 || m.FullCount != 1 {
		t.Errorf("young/full counts: %d / %d", m.YoungCount, m.FullCount)
	}
	if m.ExplicitGCCount != 1 {
		t.Errorf("expected 1 explicit GC, got %d", m.ExplicitGCCount)
	}
	if m.HumongousCount != 5 {
		t.Errorf("expected 5 humongous, got %d", m.HumongousCount)
	}
	if m.Under25ms != 2 {
		t.Errorf("expected 2 under 25ms, got %d", m.Under25ms)
	}
	if m.From25To50 != 1 {
		t.Errorf("expected 1 between 25-50ms, got %d", m.From25To50)
	}
	if m.From50To150 != 1 {
		t.Errorf("expected 1 between 50-150ms, got %d", m.From50To150)
	}
	if m.Above150ms != 1 {
		t.Errorf("expected 1 above 150ms, got %d", m.Above150ms)
	}
	if m.PauseMax != 1200*time.Millisecond {
		t.Errorf("expected 1.2s max pause, got %v", m.PauseMax)
	}
	if m.PauseP99 <= 80*time.Millisecond || m.PauseP99 > 1200*time.Millisecond {
		t.Errorf("expected interpolated p99 near 1.2s, got %v", m.PauseP99)
	}
}

func TestAllocationRate(t *testing.T) {
	mb := int64(1024 * 1024)
	events := []*model.GCEvent{
		{ID: 1, Uptime: 10 * time.Second, Duration: 10 * time.Millisecond, HeapBefore: 200 * mb, HeapAfter: 50 * mb, HeapTotal: 1000 * mb},
		{ID: 2, Uptime: 20 * time.Second, Duration: 15 * time.Millisecond, HeapBefore: 250 * mb, HeapAfter: 60 * mb, HeapTotal: 1000 * mb},
	}

	m := Analyze(events, 0)
	// Allocated 200MB (250 - 50) over 10s = 20 MB/s
	if m.AvgAllocRateMBs < 19.9 || m.AvgAllocRateMBs > 20.1 {
		t.Errorf("expected ~20 MB/s allocation rate, got %.2f", m.AvgAllocRateMBs)
	}
	// PauseSum = 25ms over 10s = 0.25%
	if m.GCOverheadPct < 0.24 || m.GCOverheadPct > 0.26 {
		t.Errorf("expected ~0.25%% GC overhead, got %.4f", m.GCOverheadPct)
	}
}
