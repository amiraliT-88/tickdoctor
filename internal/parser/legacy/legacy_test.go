package legacy

import (
	"strings"
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

func TestParseLegacyCommandLine(t *testing.T) {
	log := `CommandLine flags: -XX:InitialHeapSize=2147483648 -XX:MaxHeapSize=4294967296 -XX:+PrintGC -XX:+PrintGCDetails -XX:+UseParallelGC -XX:+AlwaysPreTouch
2026-11-15T18:04:10.123+0300: 10.456: [GC (Allocation Failure) [PSYoungGen: 524288K->65536K(614400K)] 524288K->65600K(2097152K), 0.0152340 secs] [Times: user=0.06 sys=0.00, real=0.02 secs]
2026-11-15T18:05:00.000+0300: 60.123: [Full GC (System.gc()) [PSYoungGen: 65536K->0K(614400K)] [ParOldGen: 500000K->350000K(1482752K)] 565536K->350000K(2097152K), [Metaspace: 35000K->35000K(1081344K)], 0.8523100 secs] [Times: user=3.20 sys=0.01, real=0.85 secs]
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if p.JVM.Collector != model.CollectorParallel {
		t.Errorf("expected Parallel collector, got %s", p.JVM.Collector)
	}
	if p.JVM.HeapInitial != 2147483648 {
		t.Errorf("expected 2GB init heap, got %d", p.JVM.HeapInitial)
	}
	if p.JVM.HeapMax != 4294967296 {
		t.Errorf("expected 4GB max heap, got %d", p.JVM.HeapMax)
	}
	if p.JVM.AlwaysPreTouch == nil || !*p.JVM.AlwaysPreTouch {
		t.Errorf("expected AlwaysPreTouch true, got %v", p.JVM.AlwaysPreTouch)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	ev0 := events[0]
	if ev0.Type != model.GCTypeYoung {
		t.Errorf("expected Young GC, got %s", ev0.Type)
	}
	if ev0.Duration < 15*time.Millisecond || ev0.Duration > 16*time.Millisecond {
		t.Errorf("expected ~15.2ms duration, got %v", ev0.Duration)
	}
	if ev0.HeapBefore != 524288*1024 || ev0.HeapAfter != 65600*1024 {
		t.Errorf("heap mismatch: %d -> %d", ev0.HeapBefore, ev0.HeapAfter)
	}

	ev1 := events[1]
	if ev1.Type != model.GCTypeFull {
		t.Errorf("expected Full GC, got %s", ev1.Type)
	}
	if !strings.Contains(ev1.Reason, "System.gc()") {
		t.Errorf("expected System.gc() reason, got %q", ev1.Reason)
	}
	if ev1.Duration < 850*time.Millisecond {
		t.Errorf("expected >850ms duration, got %v", ev1.Duration)
	}
}

func TestParseLegacyG1(t *testing.T) {
	log := `2026-11-15T18:04:12.123+0300: 12.456: [GC pause (G1 Evacuation Pause) (young), 0.0142340 secs]
 [Eden: 124.0M(124.0M)->0.0B(120.0M) Survivors: 0.0B->4096.0K Heap: 124.0M(4096.0M)->42.0M(4096.0M)] [Times: user=0.05 sys=0.00, real=0.01 secs]
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != model.GCTypeYoung {
		t.Errorf("expected young G1 pause, got %s", ev.Type)
	}
	if ev.HeapBefore != 124*1024*1024 || ev.HeapAfter != 42*1024*1024 {
		t.Errorf("heap mismatch: %d -> %d", ev.HeapBefore, ev.HeapAfter)
	}
}

func TestParseLegacyCMS(t *testing.T) {
	log := `2026-11-15T18:04:10.123+0300: 10.456: [GC (Allocation Failure) [ParNew: 272640K->30336K(306688K), 0.0182340 secs] 500000K->350000K(2097152K), 0.0183000 secs]
2026-11-15T18:05:10.000+0300: 70.123: [Full GC (Concurrent Mode Failure) [CMS: 1790464K->1200000K(1790464K), 2.5000000 secs] 2097152K->1250000K(2097152K), 2.5010000 secs]
`
	p := NewParser()
	events, err := p.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if p.JVM.Collector != model.CollectorCMS {
		t.Errorf("expected CMS collector, got %s", p.JVM.Collector)
	}

	ev1 := events[1]
	if ev1.Type != model.GCTypeFull {
		t.Errorf("expected Full GC on Concurrent Mode Failure, got %s", ev1.Type)
	}
	if !strings.Contains(ev1.Reason, "Concurrent Mode Failure") {
		t.Errorf("expected Concurrent Mode Failure in reason, got %q", ev1.Reason)
	}
	if ev1.Duration < 2500*time.Millisecond {
		t.Errorf("expected ~2.5s duration, got %v", ev1.Duration)
	}
}
