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

func TestParseQuotedTmuxAndBatchVars(t *testing.T) {
	sh := `#!/bin/bash
SESSION="lobby"
tmux new-session -d -s "$SESSION" \
"java -Xms2048M -Xmx2048M -XX:+UseG1GC -XX:+DisableExplicitGC -XX:+AlwaysPreTouch -jar server.jar --nogui"
`
	parsedSh := ParseCommandLine(sh)
	if parsedSh.HeapMax != 2048*1024*1024 || parsedSh.Collector != "G1" {
		t.Errorf("failed to parse quoted tmux command: %+v", parsedSh)
	}
	if parsedSh.PreTouch == nil || !*parsedSh.PreTouch || parsedSh.DisableGC == nil || !*parsedSh.DisableGC {
		t.Errorf("expected PreTouch and DisableGC true in quoted tmux command")
	}

	bat := `@echo off
set /A memory=1024
java -Xms%memory%M -Xmx%memory%M -XX:+UseG1GC -jar server.jar
`
	parsedBat := ParseCommandLine(bat)
	if parsedBat.HeapMin != 1024*1024*1024 || parsedBat.HeapMax != 1024*1024*1024 {
		t.Errorf("failed to expand batch %%memory%% var: min=%d max=%d", parsedBat.HeapMin, parsedBat.HeapMax)
	}
}
