package rules

import (
	"testing"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/model"
)

func TestEngineLoadAndEvaluate(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	if len(engine.rules) < 5 {
		t.Fatalf("expected at least 5 builtin rules, loaded %d", len(engine.rules))
	}

	m := analyzer.Metrics{
		ExplicitGCCount:    2,
		EvacuationFailures: 1,
		HumongousCount:     50,
	}

	pretouchFalse := false
	jvm := model.JVMInfo{
		Collector:      model.CollectorG1,
		HeapInitial:    2048 * 1024 * 1024,
		HeapMax:        4096 * 1024 * 1024,
		AlwaysPreTouch: &pretouchFalse,
	}

	findings := engine.Evaluate(m, jvm)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(findings))
	}

	foundExplicit := false
	for _, f := range findings {
		if f.Rule.ID == "GC-001" {
			foundExplicit = true
		}
	}
	if !foundExplicit {
		t.Errorf("expected GC-001 finding for System.gc()")
	}
}

func TestEngineEvaluateFlags(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	flags := []string{
		"-Xms2G",
		"-Xmx16G",
		"-XX:+UseG1GC",
		"-XX:+UseParallelGC",
		"-XX:+CMSIncrementalMode",
	}

	// 16GB host, 16GB heap, Java 21
	findings := engine.EvaluateFlags(flags, 2*1024*1024*1024, 16*1024*1024*1024, nil, nil, 16*1024*1024*1024, 21)

	ids := make(map[string]bool)
	for _, f := range findings {
		ids[f.Rule.ID] = true
	}

	if !ids["LINT-001"] {
		t.Errorf("expected LINT-001 for conflicting collectors")
	}
	if !ids["LINT-002"] {
		t.Errorf("expected LINT-002 for obsolete CMSIncrementalMode")
	}
	if !ids["LINT-003"] {
		t.Errorf("expected LINT-003 for 0MB left for OS")
	}
	if !ids["GC-004"] {
		t.Errorf("expected GC-004 for heap mismatch (2G vs 16G)")
	}
}

func TestEngineZGCStallRule(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	m := analyzer.Metrics{
		ZGCAllocationStalls: 2,
	}
	jvm := model.JVMInfo{
		Collector: model.CollectorZGC,
	}

	findings := engine.Evaluate(m, jvm)
	foundZGC := false
	for _, f := range findings {
		if f.Rule.ID == "ZGC-001" {
			foundZGC = true
		}
	}
	if !foundZGC {
		t.Errorf("expected ZGC-001 finding when Collector is ZGC and stalls > 0")
	}
}

func TestEngineDuplicateCollectorAndHeadroom(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	// Duplicate -XX:+UseG1GC must not trigger LINT-001; 15G heap on 16G RAM leaves 1024MB (<1536MB) -> LINT-003.
	flags := []string{"-Xms15G", "-Xmx15G", "-XX:+UseG1GC", "-XX:+UseG1GC", "-XX:+AlwaysPreTouch", "-XX:+DisableExplicitGC"}
	findings := engine.EvaluateFlags(flags, 15*1024*1024*1024, 15*1024*1024*1024, ptrBool(true), ptrBool(true), 16*1024*1024*1024, 21)

	ids := make(map[string]bool)
	for _, f := range findings {
		ids[f.Rule.ID] = true
	}

	if ids["LINT-001"] {
		t.Errorf("did not expect LINT-001 for duplicated identical collector flag")
	}
	if !ids["LINT-003"] {
		t.Errorf("expected LINT-003 when OS headroom is below 1536 MB")
	}
}

func ptrBool(v bool) *bool {
	return &v
}

func TestEngineNewDiagnosticRulesAndSmartRegion(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	m := analyzer.Metrics{
		HumongousCount:    30,
		MetaspaceGCCount:  3,
		SuspectedLeak:     true,
		PostGCHeapSlopeMB: 42.5,
		MaxHeapAfter:      3600 * 1024 * 1024,
		HeapCapacity:      4096 * 1024 * 1024,
		AvgAllocRateMBs:   750.0,
	}
	jvm := model.JVMInfo{
		Collector:  model.CollectorG1,
		RegionSize: 4 * 1024 * 1024,
	}

	findings := engine.Evaluate(m, jvm)
	got := make(map[string]Finding)
	for _, f := range findings {
		got[f.Rule.ID] = f
	}

	for _, id := range []string{"GC-003", "GC-006", "GC-007", "GC-008"} {
		if _, ok := got[id]; !ok {
			t.Errorf("expected rule %s to trigger", id)
		}
	}
	if rec := got["GC-003"].Recommendation; rec != "set -XX:G1HeapRegionSize=8M (current: 4M)" {
		t.Errorf("unexpected smart G1HeapRegionSize recommendation: %q", rec)
	}

	shenFindings := engine.Evaluate(analyzer.Metrics{ShenandoahDegenerated: 2}, model.JVMInfo{Collector: model.CollectorShenandoah})
	foundShen := false
	for _, f := range shenFindings {
		if f.Rule.ID == "SHEN-001" {
			foundShen = true
		}
	}
	if !foundShen {
		t.Errorf("expected SHEN-001 finding for Shenandoah degenerated pauses")
	}
}
