package text

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

func TestRender(t *testing.T) {
	jvm := model.JVMInfo{
		Collector:   model.CollectorG1,
		FullVersion: "21.0.3+9",
		HeapMax:     4096 * 1024 * 1024,
	}
	m := analyzer.Metrics{
		TotalEvents: 10,
		PauseAvg:    20 * time.Millisecond,
		PauseMax:    120 * time.Millisecond,
		Under25ms:   8,
		From50To150: 2,
	}
	findings := []rules.Finding{
		{
			Rule: rules.Rule{
				ID:          "GC-001",
				Title:       "Explicit System.gc()",
				Severity:    rules.SeverityCritical,
				Explanation: "Forces full STW pause.",
			},
			Evidence:       "2 calls",
			Recommendation: "add -XX:+DisableExplicitGC",
		},
	}

	var buf bytes.Buffer
	Render(&buf, jvm, m, findings)
	out := buf.String()

	if !strings.Contains(out, "TickDoctor GC Diagnostic Report") {
		t.Errorf("missing header in output")
	}
	if !strings.Contains(out, "CRITICAL") || !strings.Contains(out, "DisableExplicitGC") {
		t.Errorf("missing critical finding in output")
	}
}

func TestRenderCorrelation(t *testing.T) {
	corr := &correlate.Report{
		TotalLagEvents:      5,
		CorrelatedLagEvents: 4,
		LagCorrelationRatio: 80.0,
		TotalLagDuration:    10 * time.Second,
		ExplainedGCDuration: 8 * time.Second,
		ExplainedRatio:      80.0,
		Verdict:             correlate.VerdictGCPrimary,
		VerdictTitle:        "GC is Primary Cause of Server Lag",
		VerdictMessage:      "80% of lag spikes coincide with GC pauses.",
	}

	var buf bytes.Buffer
	RenderCorrelation(&buf, corr)
	out := buf.String()

	if !strings.Contains(out, "Server Lag Correlation") {
		t.Errorf("missing correlation header")
	}
	if !strings.Contains(out, "GC_PRIMARY") {
		t.Errorf("missing verdict in output")
	}
}

