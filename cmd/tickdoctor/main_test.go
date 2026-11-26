package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteAnalysisJSONAndMarkdown(t *testing.T) {
	gcLog := filepath.Join("..", "..", "testdata", "logs", "sample_java21_paper.log")
	serverLog := filepath.Join("..", "..", "testdata", "serverlogs", "sample_latest.log")
	cmdScript := filepath.Join("..", "..", "testdata", "cmdlines", "dirty_flags.sh")

	outJSON := filepath.Join(t.TempDir(), "report.json")
	code := executeAnalysis(gcLog, serverLog, cmdScript, "json", outJSON, false, true)
	if code != 2 {
		t.Fatalf("expected exit code 2 for critical findings, got %d", code)
	}

	data, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if _, ok := parsed["correlation"]; !ok {
		t.Errorf("expected correlation field in JSON report")
	}

	outMD := filepath.Join(t.TempDir(), "report.md")
	code = executeAnalysis(gcLog, serverLog, "", "markdown", outMD, true, true)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	mdBytes, err := os.ReadFile(outMD)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mdBytes), "# TickDoctor GC Diagnostic Report") {
		t.Errorf("missing markdown header")
	}
}

func TestExecuteAnalysisErrors(t *testing.T) {
	if code := executeAnalysis("", "", "", "xml", "", false, true); code != 3 {
		t.Errorf("expected exit code 3 for invalid format, got %d", code)
	}
	if code := executeAnalysis(filepath.Join(t.TempDir(), "missing.log"), "", "", "text", "", false, true); code != 3 {
		t.Errorf("expected exit code 3 for missing log, got %d", code)
	}
}

func TestExecuteLint(t *testing.T) {
	healthy := filepath.Join("..", "..", "testdata", "cmdlines", "aikar_flags.sh")
	if code := executeLint(healthy, "16G", 21); code != 0 {
		t.Errorf("expected exit code 0 for healthy aikar flags, got %d", code)
	}

	dirty := filepath.Join("..", "..", "testdata", "cmdlines", "dirty_flags.sh")
	if code := executeLint(dirty, "16G", 21); code != 2 {
		t.Errorf("expected exit code 2 for dirty flags, got %d", code)
	}

	if code := executeLint("", "16G", 21); code != 3 {
		t.Errorf("expected exit code 3 when no startup script found, got %d", code)
	}
}

func TestSetupCmd(t *testing.T) {
	cmd := newSetupCmd()
	cmd.SetArgs([]string{"--java", "8"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("setup --java 8 failed: %v", err)
	}
}

func TestExtractXlogFile(t *testing.T) {
	cases := []struct {
		flag string
		want string
	}{
		{"-Xlog:gc*:file=logs/gc.log:time,uptime", "logs/gc.log"},
		{`-Xlog:gc*:file=C:\Users\asus\AppData\Local\Temp\gc.log:time,uptime`, `C:\Users\asus\AppData\Local\Temp\gc.log`},
		{`-Xlog:gc*:file="C:\Program Files\mc\gc.log":time`, `C:\Program Files\mc\gc.log`},
		{"-Xlog:gc*", ""},
	}
	for _, tc := range cases {
		if got := extractXlogFile(tc.flag); got != tc.want {
			t.Errorf("extractXlogFile(%q) = %q, want %q", tc.flag, got, tc.want)
		}
	}
}

func TestExecuteDiff(t *testing.T) {
	beforeLog := filepath.Join("..", "..", "testdata", "logs", "sample_java8_parallel.log")
	afterLog := filepath.Join("..", "..", "testdata", "logs", "sample_java21_paper.log")

	if code := executeDiff(beforeLog, afterLog, "text"); code != 0 {
		t.Errorf("expected exit code 0 for text diff, got %d", code)
	}
	if code := executeDiff(beforeLog, afterLog, "json"); code != 0 {
		t.Errorf("expected exit code 0 for json diff, got %d", code)
	}
	if code := executeDiff(beforeLog, afterLog, "xml"); code != 3 {
		t.Errorf("expected exit code 3 for invalid diff format, got %d", code)
	}
	if code := executeDiff(filepath.Join(t.TempDir(), "missing.log"), afterLog, "text"); code != 3 {
		t.Errorf("expected exit code 3 for missing diff input, got %d", code)
	}
}
