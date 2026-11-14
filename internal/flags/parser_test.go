package flags

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCommandLine(t *testing.T) {
	cmd := "java -Xms4G -Xmx4096M -XX:+UseG1GC -XX:+AlwaysPreTouch -XX:+DisableExplicitGC -XX:G1HeapRegionSize=16M -jar server.jar nogui"
	parsed := ParseCommandLine(cmd)

	if parsed.HeapMin != 4*1024*1024*1024 {
		t.Errorf("expected 4GB min heap, got %d", parsed.HeapMin)
	}
	if parsed.HeapMax != 4*1024*1024*1024 {
		t.Errorf("expected 4GB max heap, got %d", parsed.HeapMax)
	}
	if parsed.Collector != "G1" {
		t.Errorf("expected G1, got %s", parsed.Collector)
	}
	if parsed.PreTouch == nil || !*parsed.PreTouch {
		t.Errorf("expected PreTouch true, got %v", parsed.PreTouch)
	}
	if parsed.DisableGC == nil || !*parsed.DisableGC {
		t.Errorf("expected DisableGC true, got %v", parsed.DisableGC)
	}
	if parsed.RegionSize != 16*1024*1024 {
		t.Errorf("expected 16MB region size, got %d", parsed.RegionSize)
	}
}

func TestParseScriptFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "start.sh")
	script := `#!/bin/bash
java -Xms2048M -Xmx8G \
  -XX:+UseG1GC \
  -jar paper.jar --nogui
`
	if err := os.WriteFile(tmp, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseScriptOrString(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.HeapMin != 2048*1024*1024 {
		t.Errorf("expected 2048MB min heap, got %d", parsed.HeapMin)
	}
	if parsed.HeapMax != 8*1024*1024*1024 {
		t.Errorf("expected 8GB max heap, got %d", parsed.HeapMax)
	}
	if parsed.Collector != "G1" {
		t.Errorf("expected G1, got %s", parsed.Collector)
	}
}

func TestParseIgnoresComments(t *testing.T) {
	script := `#!/bin/bash
# -Xmx2G -XX:+UseParallelGC
:: batch comment -XX:+UseConcMarkSweepGC
REM legacy comment -Xincgc
-Xms8G -Xmx8G # inline comment -XX:+UseSerialGC
-XX:+UseG1GC
`
	parsed := ParseCommandLine(script)
	if parsed.HeapMax != 8*1024*1024*1024 {
		t.Errorf("expected 8GB max heap, got %d", parsed.HeapMax)
	}
	if parsed.Collector != "G1" {
		t.Errorf("expected G1 collector, got %s", parsed.Collector)
	}
	if len(parsed.AllFlags) != 3 {
		t.Errorf("expected exactly 3 active flags, got %v", parsed.AllFlags)
	}
}
