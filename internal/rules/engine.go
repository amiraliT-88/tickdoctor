package rules

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

type Engine struct {
	rules []Rule
}

func NewEngine() (*Engine, error) {
	e := &Engine{}
	if err := e.loadBuiltin(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Engine) loadBuiltin() error {
	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := builtinFS.ReadFile("builtin/" + entry.Name())
		if err != nil {
			return err
		}
		var r Rule
		if err := yaml.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("parsing rule %s: %w", entry.Name(), err)
		}
		e.rules = append(e.rules, r)
	}
	return nil
}

func (e *Engine) Evaluate(m analyzer.Metrics, jvm model.JVMInfo) []Finding {
	var findings []Finding

	for _, r := range e.rules {
		if r.Collector != "All" && string(jvm.Collector) != r.Collector && jvm.Collector != model.CollectorUnknown {
			continue
		}

		triggered := false
		evidence := ""
		recommendation := r.Recommendation

		switch r.ConditionType {
		case "explicit_gc_count":
			if float64(m.ExplicitGCCount) >= r.Threshold {
				triggered = true
				evidence = fmt.Sprintf("%d Stop-The-World Full GC pause(s) caused by System.gc()", m.ExplicitGCCount)
			}
		case "evacuation_failure_count":
			if float64(m.EvacuationFailures) >= r.Threshold {
				triggered = true
				evidence = fmt.Sprintf("%d evacuation failure(s) recorded (to-space exhausted)", m.EvacuationFailures)
			}
		case "humongous_count":
			if float64(m.HumongousCount) >= r.Threshold {
				triggered = true
				evidence = fmt.Sprintf("%d humongous allocation region(s) detected", m.HumongousCount)
				if jvm.RegionSize > 0 && jvm.RegionSize < 32*1024*1024 {
					currentMB := jvm.RegionSize / (1024 * 1024)
					targetMB := currentMB * 2
					if targetMB < 8 {
						targetMB = 8
					}
					if targetMB > 32 {
						targetMB = 32
					}
					recommendation = fmt.Sprintf("set -XX:G1HeapRegionSize=%dM (current: %dM)", targetMB, currentMB)
				}
			}
		case "heap_mismatch":
			if jvm.HeapInitial > 0 && jvm.HeapMax > 0 && jvm.HeapInitial != jvm.HeapMax {
				triggered = true
				evidence = fmt.Sprintf("initial heap (%d MB) differs from max heap (%d MB)",
					jvm.HeapInitial/(1024*1024), jvm.HeapMax/(1024*1024))
			}
		case "pretouch_disabled":
			if jvm.AlwaysPreTouch != nil && !*jvm.AlwaysPreTouch {
				triggered = true
				evidence = "Pre-touch memory allocation is reported as Disabled in JVM header"
			}
		case "java8_parallel":
			if jvm.MajorVersion == 8 && (jvm.Collector == model.CollectorParallel || jvm.Collector == model.CollectorUnknown) && (m.FullCount > 0 || m.Above150ms > 0) {
				triggered = true
				evidence = fmt.Sprintf("Java 8 Parallel collector caused %d Full GC(s) and %d pause(s) >150ms", m.FullCount, m.Above150ms)
			}
		case "cms_failure":
			if m.CMSFailures >= int(r.Threshold) {
				triggered = true
				evidence = fmt.Sprintf("%d CMS Concurrent Mode Failure(s) triggered emergency Full GC", m.CMSFailures)
			}
		case "zgc_allocation_stall":
			if m.ZGCAllocationStalls >= int(r.Threshold) {
				triggered = true
				evidence = fmt.Sprintf("%d ZGC allocation stall(s) detected (thread paused for memory reclamation)", m.ZGCAllocationStalls)
			}
		case "metaspace_gc_count":
			if float64(m.MetaspaceGCCount) >= r.Threshold {
				triggered = true
				evidence = fmt.Sprintf("%d Metadata GC Threshold pause(s) triggered by classloading/Metaspace growth", m.MetaspaceGCCount)
			}
		case "suspected_leak":
			if m.SuspectedLeak {
				triggered = true
				heapPct := 0.0
				if m.HeapCapacity > 0 {
					heapPct = float64(m.MaxHeapAfter) / float64(m.HeapCapacity) * 100.0
				}
				evidence = fmt.Sprintf("post-GC heap climbing at +%.1f MB/GC (peaked at %.1f%% of heap capacity)",
					m.PostGCHeapSlopeMB, heapPct)
			}
		case "high_alloc_rate":
			if m.AvgAllocRateMBs >= r.Threshold {
				triggered = true
				evidence = fmt.Sprintf("average allocation rate is %.1f MB/s (threshold: %.0f MB/s)",
					m.AvgAllocRateMBs, r.Threshold)
			}
		case "shenandoah_degenerated":
			if m.ShenandoahDegenerated >= int(r.Threshold) {
				triggered = true
				evidence = fmt.Sprintf("%d Shenandoah Degenerated GC pause(s) detected", m.ShenandoahDegenerated)
			}
		case "unsafe_memory":
			if jvm.PhysicalMemory > 0 && jvm.HeapMax > 0 {
				if jvm.HeapMax > jvm.PhysicalMemory {
					triggered = true
					evidence = fmt.Sprintf("Max heap (%d MB) exceeds host RAM (%d MB)",
						jvm.HeapMax/(1024*1024), jvm.PhysicalMemory/(1024*1024))
				} else if (jvm.PhysicalMemory - jvm.HeapMax) < 1536*1024*1024 {
					triggered = true
					evidence = fmt.Sprintf("Only %d MB left for OS and off-heap memory (recommended >= 1536 MB)",
						(jvm.PhysicalMemory-jvm.HeapMax)/(1024*1024))
				}
			}
		}

		if triggered {
			findings = append(findings, Finding{
				Rule:           r,
				Evidence:       evidence,
				Recommendation: recommendation,
			})
		}
	}

	return findings
}

func (e *Engine) EvaluateFlags(allFlags []string, heapMin, heapMax int64, preTouch, disableGC *bool, systemRAM int64, javaVersion int) []Finding {
	var findings []Finding

	for _, r := range e.rules {
		triggered := false
		evidence := ""

		switch r.ConditionType {
		case "conflicting_collectors":
			collectors := findCollectorFlags(allFlags)
			if len(collectors) >= int(r.Threshold) {
				triggered = true
				evidence = fmt.Sprintf("Multiple collectors specified: %s", strings.Join(collectors, ", "))
			}

		case "removed_flags":
			if javaVersion >= 17 {
				obsolete := findObsoleteFlags(allFlags)
				if len(obsolete) >= int(r.Threshold) {
					triggered = true
					evidence = fmt.Sprintf("Unsupported Java 17+ flags detected: %s", strings.Join(obsolete, ", "))
				}
			}

		case "unsafe_memory":
			if systemRAM > 0 && heapMax > 0 {
				if heapMax > systemRAM {
					triggered = true
					evidence = fmt.Sprintf("Max heap (%d MB) exceeds host RAM (%d MB)",
						heapMax/(1024*1024), systemRAM/(1024*1024))
				} else if (systemRAM - heapMax) < 1536*1024*1024 {
					triggered = true
					evidence = fmt.Sprintf("Only %d MB left for OS and off-heap memory (recommended >= 1536 MB)",
						(systemRAM-heapMax)/(1024*1024))
				}
			}

		case "heap_mismatch":
			if heapMin > 0 && heapMax > 0 && heapMin != heapMax {
				triggered = true
				evidence = fmt.Sprintf("-Xms (%d MB) differs from -Xmx (%d MB)",
					heapMin/(1024*1024), heapMax/(1024*1024))
			}

		case "pretouch_disabled":
			if preTouch == nil || !*preTouch {
				triggered = true
				evidence = "-XX:+AlwaysPreTouch is missing from startup flags"
			}

		case "explicit_gc_count":
			if disableGC == nil || !*disableGC {
				triggered = true
				evidence = "-XX:+DisableExplicitGC is missing from startup flags"
			}
		}

		if triggered {
			findings = append(findings, Finding{
				Rule:           r,
				Evidence:       evidence,
				Recommendation: r.Recommendation,
			})
		}
	}

	return findings
}

func findCollectorFlags(flags []string) []string {
	var found []string
	seen := make(map[string]bool)
	known := []string{
		"-XX:+UseG1GC",
		"-XX:+UseParallelGC",
		"-XX:+UseConcMarkSweepGC",
		"-XX:+UseZGC",
		"-XX:+UseShenandoahGC",
		"-XX:+UseSerialGC",
	}
	for _, f := range flags {
		if seen[f] {
			continue
		}
		for _, k := range known {
			if f == k {
				found = append(found, f)
				seen[f] = true
				break
			}
		}
	}
	return found
}

func findObsoleteFlags(flags []string) []string {
	var found []string
	seen := make(map[string]bool)
	obsoletePrefixes := []string{
		"-XX:+UseConcMarkSweepGC",
		"-XX:+CMSIncrementalMode",
		"-XX:+CMSClassUnloadingEnabled",
		"-Xincgc",
		"-XX:MaxPermSize=",
		"-XX:PermSize=",
		"-XX:+UseParNewGC",
	}
	for _, f := range flags {
		if seen[f] {
			continue
		}
		for _, obs := range obsoletePrefixes {
			if strings.HasPrefix(f, obs) {
				found = append(found, f)
				seen[f] = true
				break
			}
		}
	}
	return found
}
