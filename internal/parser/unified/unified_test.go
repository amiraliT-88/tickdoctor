package unified

import (
	"strings"
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

func TestParseHeader(t *testing.T) {
	log := `
[0.002s][info][gc] Using G1
[0.002s][info][gc,init] Version: 21.0.3+9-LTS (release)
[0.002s][info][gc,init] CPUs: 8 total, 8 available
[0.002s][info][gc,init] Memory: 16384M
[0.002s][info][gc,init] Heap Region Size: 4M
[0.002s][info][gc,init] Heap Min Capacity: 4096M
[0.002s][info][gc,init] Heap Initial Capacity: 4096M
[0.002s][info][gc,init] Heap Max Capacity: 4096M
[0.002s][info][gc,init] Pre-touch: Enabled
`
	p := NewParser()
	_, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if p.JVM.Collector != model.CollectorG1 {
		t.Errorf("expected G1, got %v", p.JVM.Collector)
	}
	if p.JVM.MajorVersion != 21 {
		t.Errorf("expected Java 21, got %d", p.JVM.MajorVersion)
	}
	if p.JVM.CPUs != 8 {
		t.Errorf("expected 8 CPUs, got %d", p.JVM.CPUs)
	}
	if p.JVM.PhysicalMemory != 16384*1024*1024 {
		t.Errorf("expected 16GB RAM, got %d", p.JVM.PhysicalMemory)
	}
	if p.JVM.HeapMax != 4096*1024*1024 {
		t.Errorf("expected 4GB Max Heap, got %d", p.JVM.HeapMax)
	}
	if p.JVM.AlwaysPreTouch == nil || !*p.JVM.AlwaysPreTouch {
		t.Errorf("expected Pre-touch true, got %v", p.JVM.AlwaysPreTouch)
	}
}

func TestParseEvents(t *testing.T) {
	log := `
[2026-11-15T18:04:12.123+0300][12.456s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 124M->42M(4096M) 14.234ms
[2026-11-15T18:04:15.000+0300][15.000s][info][gc,heap] GC(1) Humongous regions: 0->4
[2026-11-15T18:04:15.010+0300][15.010s][info][gc     ] GC(1) Pause Young (Normal) (G1 Evacuation Pause) 500M->200M(4096M) 22,500ms
[2026-11-15T18:05:00.100+0300][60.100s][info][gc     ] GC(2) Pause Full (System.gc()) 3200M->850M(4096M) 1845.120ms
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	ev0 := events[0]
	if ev0.Type != model.GCTypeYoung || ev0.Duration != 14234*time.Microsecond {
		t.Errorf("ev0 mismatch: type=%v dur=%v", ev0.Type, ev0.Duration)
	}
	if ev0.HeapBefore != 124*1024*1024 || ev0.HeapAfter != 42*1024*1024 {
		t.Errorf("ev0 heap mismatch: %d -> %d", ev0.HeapBefore, ev0.HeapAfter)
	}

	ev1 := events[1]
	if ev1.HumongousCount != 4 {
		t.Errorf("expected 4 humongous regions, got %d", ev1.HumongousCount)
	}
	if ev1.Duration != 22500*time.Microsecond {
		t.Errorf("expected 22.5ms duration, got %v", ev1.Duration)
	}

	ev2 := events[2]
	if ev2.Type != model.GCTypeFull {
		t.Errorf("expected Full GC, got %v", ev2.Type)
	}
	if !strings.Contains(ev2.Reason, "System.gc()") {
		t.Errorf("expected System.gc() reason, got %q", ev2.Reason)
	}
	if ev2.Duration < 1845*time.Millisecond {
		t.Errorf("expected >1.8s duration, got %v", ev2.Duration)
	}
}

func TestParseZGC(t *testing.T) {
	log := `
[0.002s][info][gc] Using The Z Garbage Collector
[0.002s][info][gc,init] Version: 21.0.3+9-LTS (release)
[2026-11-15T18:04:10.000+0300][10.000s][info][gc,phases] GC(0) Pause Mark Start 0.050ms
[2026-11-15T18:04:10.010+0300][10.010s][info][gc,phases] GC(0) Pause Mark End 0.080ms
[2026-11-15T18:04:12.000+0300][12.000s][info][gc       ] GC(1) Allocation Stall (Server thread) 15.500ms
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if p.JVM.Collector != model.CollectorZGC {
		t.Errorf("expected CollectorZGC, got %v", p.JVM.Collector)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 ZGC events, got %d", len(events))
	}

	if events[0].Duration < 120*time.Microsecond || events[0].Duration > 140*time.Microsecond {
		t.Errorf("expected ~130µs duration, got %v", events[0].Duration)
	}

	if events[1].Duration != 15500*time.Microsecond {
		t.Errorf("expected 15.5ms stall, got %v", events[1].Duration)
	}
	if !strings.Contains(events[1].Reason, "Allocation Stall") {
		t.Errorf("expected Allocation Stall in reason, got %s", events[1].Reason)
	}
}

func TestParseGenerationalZGC(t *testing.T) {
	log := `
[2026-11-15T18:04:00.000+0300][0.002s][1234][info   ][gc] Using The Z Garbage Collector
[2026-11-15T18:04:00.002+0300][0.004s][1234][info   ][gc,init] Version: 21.0.5+11-LTS (release)
[2026-11-15T18:04:10.000+0300][10.000s][1234][info   ][gc,phases] GC(0) Y: Pause Mark Start 0.025ms
[2026-11-15T18:04:10.010+0300][10.010s][1234][info   ][gc,phases] GC(0) Y: Pause Mark End 0.035ms
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if p.JVM.MajorVersion != 21 || p.JVM.Collector != model.CollectorZGC {
		t.Fatalf("unexpected JVM info: %+v", p.JVM)
	}
	if len(events) != 1 || p.EventCount() != 1 || p.LastEvent() == nil {
		t.Fatalf("expected 1 ZGC event, got %d", len(events))
	}
	if events[0].Duration < 55*time.Microsecond || events[0].Duration > 65*time.Microsecond {
		t.Errorf("expected ~60µs pause, got %v", events[0].Duration)
	}
}

func TestParseShenandoah(t *testing.T) {
	log := `
[0.002s][info][gc] Using Shenandoah
[2026-11-15T18:04:10.000+0300][10.000s][info][gc] GC(0) Pause Init Mark 0.120ms
[2026-11-15T18:04:10.050+0300][10.050s][info][gc] GC(0) Pause Final Mark 0.240ms
[2026-11-15T18:05:00.000+0300][60.000s][info][gc] GC(1) Pause Degenerated GC (Mark) 2048M->1024M(4096M) 412.500ms
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if p.JVM.Collector != model.CollectorShenandoah {
		t.Fatalf("expected CollectorShenandoah, got %v", p.JVM.Collector)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 Shenandoah events, got %d", len(events))
	}
	if !strings.Contains(events[0].Reason, "Shenandoah Pause") {
		t.Errorf("expected Shenandoah Pause reason, got %q", events[0].Reason)
	}
	if events[1].Type != model.GCTypeFull || !strings.Contains(events[1].Reason, "Degenerated") {
		t.Errorf("expected degenerated pause mapped to Full GC, got type=%s reason=%q", events[1].Type, events[1].Reason)
	}
	if events[1].Duration != 412500*time.Microsecond {
		t.Errorf("expected 412.5ms duration, got %v", events[1].Duration)
	}
}
