package tui

import "github.com/charmbracelet/lipgloss"

var (
	subtleColor   = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#777777"}
	primaryColor  = lipgloss.AdaptiveColor{Light: "#007acc", Dark: "#00add8"}
	criticalColor = lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#ff5555"}
	warningColor  = lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f59e0b"}
	successColor  = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#22c55e"}
	infoColor     = lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"}

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				Padding(0, 2)

	subtleStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	badgeCritical = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(criticalColor).
			Padding(0, 1)

	badgeWarning = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(warningColor).
			Padding(0, 1)

	badgeInfo = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(infoColor).
			Padding(0, 1)

	badgeHealthy = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(successColor).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)
)
