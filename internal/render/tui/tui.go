package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	jvm             model.JVMInfo
	metrics         analyzer.Metrics
	findings        []rules.Finding
	events          []*model.GCEvent
	corr            *correlate.Report
	activeTab       int
	table           table.Model
	findingSelected int
	width           int
	height          int
}

func NewModel(jvm model.JVMInfo, metrics analyzer.Metrics, findings []rules.Finding, events []*model.GCEvent) Model {
	return NewModelWithCorrelation(jvm, metrics, findings, events, nil)
}

func NewModelWithCorrelation(jvm model.JVMInfo, metrics analyzer.Metrics, findings []rules.Finding, events []*model.GCEvent, corr *correlate.Report) Model {
	columns := []table.Column{
		{Title: "ID", Width: 6},
		{Title: "Type", Width: 10},
		{Title: "Duration", Width: 12},
		{Title: "Heap Before -> After", Width: 26},
		{Title: "Reason", Width: 32},
	}

	rows := make([]table.Row, 0, len(events))
	for _, ev := range events {
		heapStr := "-"
		if ev.HeapTotal > 0 || ev.HeapBefore > 0 {
			heapStr = fmt.Sprintf("%dM -> %dM (%dM)",
				ev.HeapBefore/(1024*1024),
				ev.HeapAfter/(1024*1024),
				ev.HeapTotal/(1024*1024),
			)
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", ev.ID),
			string(ev.Type),
			formatDuration(ev.Duration),
			heapStr,
			ev.Reason,
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(14),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return Model{
		jvm:       jvm,
		metrics:   metrics,
		findings:  findings,
		events:    events,
		corr:      corr,
		activeTab: 0,
		table:     t,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(max(6, msg.Height-10))

	case tea.KeyMsg:
		numTabs := 3
		if m.corr != nil {
			numTabs = 4
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "l", "right":
			m.activeTab = (m.activeTab + 1) % numTabs
			return m, nil
		case "shift+tab", "h", "left":
			m.activeTab = (m.activeTab + numTabs - 1) % numTabs
			return m, nil
		case "1":
			m.activeTab = 0
			return m, nil
		case "2":
			m.activeTab = 1
			return m, nil
		case "3":
			m.activeTab = 2
			return m, nil
		case "4":
			if numTabs == 4 {
				m.activeTab = 3
			}
			return m, nil
		case "j", "down":
			if m.activeTab == 2 && len(m.findings) > 0 {
				if m.findingSelected < len(m.findings)-1 {
					m.findingSelected++
				}
				return m, nil
			}
		case "k", "up":
			if m.activeTab == 2 && len(m.findings) > 0 {
				if m.findingSelected > 0 {
					m.findingSelected--
				}
				return m, nil
			}
		}
	}

	if m.activeTab == 1 {
		m.table, cmd = m.table.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder

	tabs := []string{"[1] Overview", "[2] GC Events", fmt.Sprintf("[3] Diagnoses (%d)", len(m.findings))}
	if m.corr != nil {
		tabs = append(tabs, "[4] Lag Correlation")
	}
	var renderedTabs []string
	for i, t := range tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(t))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...))
	b.WriteString("\n\n")

	switch m.activeTab {
	case 0:
		b.WriteString(m.renderOverview())
	case 1:
		b.WriteString(m.table.View())
		b.WriteString("\n\n" + subtleStyle.Render("Use ↑/↓ to navigate events table • Tab to switch view • 'q' to quit"))
	case 2:
		b.WriteString(m.renderDiagnoses())
	case 3:
		b.WriteString(m.renderCorrelation())
	}

	return b.String()
}

func (m Model) renderOverview() string {
	var s strings.Builder

	specs := fmt.Sprintf("Collector: %s  |  Version: %s\nHeap: %d MB (Init: %d MB)  |  Region Size: %d MB",
		m.jvm.Collector,
		m.jvm.FullVersion,
		m.jvm.HeapMax/(1024*1024),
		m.jvm.HeapInitial/(1024*1024),
		m.jvm.RegionSize/(1024*1024),
	)
	jvmBox := boxStyle.Render(titleStyle.Render("JVM Environment") + "\n\n" + specs)

	pauses := fmt.Sprintf("Total Pauses: %d  |  Total Pause Time: %s\nAvg Pause: %s  |  Max Pause: %s\np50: %s  |  p90: %s  |  p95: %s  |  p99: %s",
		m.metrics.TotalEvents,
		formatDuration(m.metrics.PauseSum),
		formatDuration(m.metrics.PauseAvg),
		formatDuration(m.metrics.PauseMax),
		formatDuration(m.metrics.PauseP50),
		formatDuration(m.metrics.PauseP90),
		formatDuration(m.metrics.PauseP95),
		formatDuration(m.metrics.PauseP99),
	)
	pauseBox := boxStyle.Render(titleStyle.Render("Stop-The-World Pause Metrics") + "\n\n" + pauses)

	tickImpact := fmt.Sprintf(
		"  < 25ms   (healthy):            %d\n  25-50ms  (borderline 1 tick):  %d\n  50-150ms (stutter, 1-3 ticks): %d\n  > 150ms  (severe freeze):      %d",
		m.metrics.Under25ms,
		m.metrics.From25To50,
		m.metrics.From50To150,
		m.metrics.Above150ms,
	)
	tickBox := boxStyle.Render(titleStyle.Render("Minecraft Tick Budget Impact (50ms)") + "\n\n" + tickImpact)

	s.WriteString(lipgloss.JoinVertical(lipgloss.Left, jvmBox, pauseBox, tickBox))
	tabHint := "Tab / 1-3: Switch Tab • 'q': Quit"
	if m.corr != nil {
		tabHint = "Tab / 1-4: Switch Tab • 'q': Quit"
	}
	s.WriteString("\n\n" + subtleStyle.Render(tabHint))
	return s.String()
}

func (m Model) renderDiagnoses() string {
	if len(m.findings) == 0 {
		return boxStyle.Render(badgeHealthy.Render("HEALTHY") + "\n\nNo GC misconfigurations or stutter patterns detected.")
	}

	var leftCol strings.Builder
	for i, f := range m.findings {
		badge := badgeInfo.Render("INFO")
		if f.Rule.Severity == rules.SeverityCritical {
			badge = badgeCritical.Render("CRITICAL")
		} else if f.Rule.Severity == rules.SeverityWarning {
			badge = badgeWarning.Render("WARN")
		}

		line := fmt.Sprintf("%s %s", badge, f.Rule.Title)
		if i == m.findingSelected {
			leftCol.WriteString(selectedItemStyle.Render("▶ " + line + "\n\n"))
		} else {
			leftCol.WriteString("  " + line + "\n\n")
		}
	}

	selected := m.findings[m.findingSelected]
	var detail strings.Builder
	detail.WriteString(titleStyle.Render(fmt.Sprintf("%s (%s)", selected.Rule.Title, selected.Rule.ID)) + "\n\n")
	if selected.Evidence != "" {
		detail.WriteString(lipgloss.NewStyle().Bold(true).Render("Evidence: ") + selected.Evidence + "\n\n")
	}
	if selected.Recommendation != "" {
		detail.WriteString(lipgloss.NewStyle().Bold(true).Render("Recommended Action: ") + selected.Recommendation + "\n\n")
	}
	if selected.Rule.Explanation != "" {
		detail.WriteString(lipgloss.NewStyle().Bold(true).Render("Why: ") + selected.Rule.Explanation + "\n\n")
	}
	if selected.Rule.DocURL != "" {
		detail.WriteString(subtleStyle.Render("Reference: " + selected.Rule.DocURL))
	}

	leftWidth := 40
	rightWidth := 50
	if m.width > 0 && m.width < 100 {
		avail := max(40, m.width-12)
		leftWidth = avail * 4 / 10
		rightWidth = avail - leftWidth
	} else if m.width >= 110 {
		avail := min(140, m.width-12)
		leftWidth = avail * 4 / 10
		rightWidth = avail - leftWidth
	}

	rightCol := boxStyle.Width(rightWidth).Render(detail.String())
	cols := lipgloss.JoinHorizontal(lipgloss.Top, boxStyle.Width(leftWidth).Render(leftCol.String()), rightCol)

	return cols + "\n\n" + subtleStyle.Render("Use j/k or ↑/↓ to inspect diagnoses • Tab to switch view • 'q' to quit")
}

func (m Model) renderCorrelation() string {
	if m.corr == nil {
		return boxStyle.Render("No server log provided. Run with --server-log to correlate.")
	}

	verdictBadge := badgeHealthy.Render("GC HEALTHY")
	if m.corr.Verdict == correlate.VerdictGCPrimary {
		verdictBadge = badgeCritical.Render("GC PRIMARY CAUSE")
	} else if m.corr.Verdict == correlate.VerdictGCPartial {
		verdictBadge = badgeWarning.Render("GC CONTRIBUTING")
	} else if m.corr.Verdict == correlate.VerdictNoTimestamps {
		verdictBadge = badgeWarning.Render("NO TIMESTAMPS")
	}

	header := fmt.Sprintf("%s  %s\n\n%s", verdictBadge, titleStyle.Render(m.corr.VerdictTitle), m.corr.VerdictMessage)
	verdictBox := boxStyle.Render(header)

	stats := fmt.Sprintf("Total Lag Incidents: %d  |  Total Lag Duration: %s\nCoincided with GC: %d (%.1f%%)  |  GC Pause Share: %s (%.1f%% explained)",
		m.corr.TotalLagEvents,
		formatDuration(m.corr.TotalLagDuration),
		m.corr.CorrelatedLagEvents,
		m.corr.LagCorrelationRatio,
		formatDuration(m.corr.ExplainedGCDuration),
		m.corr.ExplainedRatio,
	)
	statsBox := boxStyle.Render(titleStyle.Render("Lag Correlation Metrics") + "\n\n" + stats)

	var incidentsList strings.Builder
	incidentsList.WriteString(titleStyle.Render("Correlated Lag Events (Showing top 5)") + "\n\n")
	if len(m.corr.Incidents) == 0 {
		incidentsList.WriteString(subtleStyle.Render("No server lag spikes overlapped with GC pauses."))
	} else {
		limit := len(m.corr.Incidents)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			inc := m.corr.Incidents[i]
			incidentsList.WriteString(fmt.Sprintf("• [%s] Server behind %s (%d ticks) - GC overlap: %s\n",
				inc.LagEvent.Time.Format("15:04:05"),
				formatDuration(inc.LagEvent.Duration),
				inc.LagEvent.TicksBehind,
				formatDuration(inc.GCPauseTotal),
			))
		}
	}
	incidentsBox := boxStyle.Render(incidentsList.String())

	return lipgloss.JoinVertical(lipgloss.Left, verdictBox, statsBox, incidentsBox) + "\n\n" + subtleStyle.Render("Tab / 1-4: Switch Tab • 'q': Quit")
}

func Run(jvm model.JVMInfo, metrics analyzer.Metrics, findings []rules.Finding, events []*model.GCEvent) error {
	return RunWithCorrelation(jvm, metrics, findings, events, nil)
}

func RunWithCorrelation(jvm model.JVMInfo, metrics analyzer.Metrics, findings []rules.Finding, events []*model.GCEvent, corr *correlate.Report) error {
	p := tea.NewProgram(NewModelWithCorrelation(jvm, metrics, findings, events, corr), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
