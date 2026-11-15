package markdown

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
)

func TestRenderMarkdown(t *testing.T) {
	jvm := model.JVMInfo{
		Collector:   model.CollectorG1,
		FullVersion: "21.0.3+9-LTS",
		HeapMax:     4096 * 1024 * 1024,
		HeapInitial: 4096 * 1024 * 1024,
		RegionSize:  16 * 1024 * 1024,
	}

	metrics := analyzer.Metrics{
		TotalEvents: 10,
		PauseAvg:    25 * time.Millisecond,
		PauseMax:    350 * time.Millisecond,
		Under25ms:   8,
		Above150ms:  1,
	}

	findings := []rules.Finding{
		{
			Rule: rules.Rule{
				ID:          "GC-001",
				Title:       "Explicit System.gc()",
				Severity:    rules.SeverityCritical,
				Explanation: "Forces Stop-The-World full pause.",
			},
			Evidence:       "Found 1 pause triggered by System.gc() from 192.168.1.50 plugin",
			Recommendation: "add -XX:+DisableExplicitGC",
		},
	}

	corr := &correlate.Report{
		TotalLagEvents:      2,
		CorrelatedLagEvents: 2,
		LagCorrelationRatio: 100.0,
		TotalLagDuration:    2 * time.Second,
		ExplainedGCDuration: 2 * time.Second,
		ExplainedRatio:      100.0,
		Verdict:             correlate.VerdictGCPrimary,
		VerdictTitle:        "GC is Primary Cause",
		VerdictMessage:      "100% correlated.",
	}

	var buf bytes.Buffer
	Render(&buf, jvm, metrics, findings, corr, Options{Redact: true})
	out := buf.String()

	if !strings.Contains(out, "# TickDoctor GC Diagnostic Report") {
		t.Errorf("missing H1 header")
	}
	if !strings.Contains(out, "```diff") || !strings.Contains(out, "+ -XX:+DisableExplicitGC") {
		t.Errorf("missing diff action plan block")
	}
	if strings.Contains(out, "192.168.1.50") {
		t.Errorf("expected IP to be redacted, got raw IP in output")
	}
	if !strings.Contains(out, "xxx.xxx.xxx.xxx") {
		t.Errorf("expected redacted placeholder xxx.xxx.xxx.xxx in output")
	}
}
