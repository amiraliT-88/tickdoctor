package json

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
)

func TestRenderJSON(t *testing.T) {
	jvm := model.JVMInfo{
		Collector:   model.CollectorG1,
		FullVersion: "21.0.3+9",
		HeapMax:     4096 * 1024 * 1024,
	}

	metrics := analyzer.Metrics{
		TotalEvents: 5,
		PauseAvg:    20 * time.Millisecond,
		Under25ms:   5,
	}

	findings := []rules.Finding{
		{
			Rule: rules.Rule{
				ID:       "GC-001",
				Title:    "Explicit GC",
				Severity: rules.SeverityCritical,
			},
			Recommendation: "add -XX:+DisableExplicitGC",
		},
	}

	corr := &correlate.Report{
		TotalLagEvents:      2,
		CorrelatedLagEvents: 2,
		LagCorrelationRatio: 100.0,
		Verdict:             correlate.VerdictGCPrimary,
		VerdictTitle:        "GC Primary",
	}

	var buf bytes.Buffer
	if err := Render(&buf, jvm, metrics, findings, corr); err != nil {
		t.Fatalf("unexpected error rendering JSON: %v", err)
	}

	var parsed FullReport
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	if parsed.JVM.Collector != "G1" {
		t.Errorf("expected collector G1, got %s", parsed.JVM.Collector)
	}
	if parsed.Metrics.TotalEvents != 5 {
		t.Errorf("expected 5 events, got %d", parsed.Metrics.TotalEvents)
	}
	if len(parsed.Findings) != 1 || parsed.Findings[0].ID != "GC-001" {
		t.Errorf("expected finding GC-001, got %+v", parsed.Findings)
	}
	if parsed.Correlation == nil || parsed.Correlation.Verdict != "GC_PRIMARY" {
		t.Errorf("expected GC_PRIMARY verdict in correlation, got %+v", parsed.Correlation)
	}
}

func TestRenderJSONEmptyFindingsNotNull(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, model.JVMInfo{Collector: model.CollectorG1}, analyzer.Metrics{}, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"findings": []`)) {
		t.Errorf("expected empty findings array [], got: %s", buf.String())
	}
}
