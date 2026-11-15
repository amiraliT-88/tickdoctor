package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIModelNavigation(t *testing.T) {
	jvm := model.JVMInfo{
		Collector:   model.CollectorG1,
		FullVersion: "21.0.3+9-LTS",
		HeapMax:     4096 * 1024 * 1024,
	}
	m := analyzer.Metrics{
		TotalEvents: 2,
		PauseAvg:    25 * time.Millisecond,
		Under25ms:   2,
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
	events := []*model.GCEvent{
		{ID: 1, Type: model.GCTypeYoung, Duration: 15 * time.Millisecond, HeapBefore: 100, HeapAfter: 40, HeapTotal: 1000},
	}

	model := NewModel(jvm, m, findings, events)

	view0 := model.View()
	if !strings.Contains(view0, "JVM Environment") || !strings.Contains(view0, "Overview") {
		t.Errorf("expected Overview content in initial view")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m2 := updated.(Model)
	if m2.activeTab != 1 {
		t.Errorf("expected activeTab = 1, got %d", m2.activeTab)
	}

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m3 := updated2.(Model)
	if m3.activeTab != 2 {
		t.Errorf("expected activeTab = 2, got %d", m3.activeTab)
	}
	view2 := m3.View()
	if !strings.Contains(view2, "Explicit GC") || !strings.Contains(view2, "DisableExplicitGC") {
		t.Errorf("expected diagnosis in view 2")
	}
}

func TestTUIWithCorrelation(t *testing.T) {
	corr := &correlate.Report{
		TotalLagEvents:      3,
		CorrelatedLagEvents: 3,
		LagCorrelationRatio: 100.0,
		Verdict:             correlate.VerdictGCPrimary,
		VerdictTitle:        "GC Primary",
	}

	model := NewModelWithCorrelation(model.JVMInfo{}, analyzer.Metrics{}, nil, nil, corr)
	view0 := model.View()
	if !strings.Contains(view0, "[4] Lag Correlation") {
		t.Errorf("expected tab 4 present when corr is non-nil")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m4 := updated.(Model)
	if m4.activeTab != 3 {
		t.Errorf("expected activeTab = 3, got %d", m4.activeTab)
	}
	view3 := m4.View()
	if !strings.Contains(view3, "Lag Correlation Metrics") {
		t.Errorf("expected correlation metrics in view 3")
	}
}

