package main

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#5aa9ff"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#6b7280"}

	colorRunning  = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#f59e0b"}
	colorSuccess  = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#4ade80"}
	colorFailed   = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f87171"}
	colorCanceled = lipgloss.AdaptiveColor{Light: "#9a3412", Dark: "#fb923c"}

	colorSelectedBg = lipgloss.AdaptiveColor{Light: "#e5e7eb", Dark: "#1f2937"}
	colorSelectedFg = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#f9fafb"}
	colorStatusBg   = lipgloss.AdaptiveColor{Light: "#f1f5f9", Dark: "#111827"}
	colorModalBg    = lipgloss.AdaptiveColor{Light: "#f8fafc", Dark: "#0f172a"}
	colorModalFg    = lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#e2e8f0"}

	titleStyle            = lipgloss.NewStyle().Bold(true)
	sectionStyle          = lipgloss.NewStyle().Foreground(colorMuted)
	comboStyle            = lipgloss.NewStyle().Foreground(colorMuted)
	selectedStyle         = lipgloss.NewStyle().Background(colorSelectedBg).Foreground(colorSelectedFg).Bold(true)
	disabledStyle         = lipgloss.NewStyle().Foreground(colorMuted).Faint(true)
	selectedDisabledStyle = lipgloss.NewStyle().Background(colorSelectedBg).Foreground(colorMuted).Faint(true)

	sidebarStyle       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	outputStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	outputContentStyle = lipgloss.NewStyle().Padding(0, 1)
	headerStyle        = lipgloss.NewStyle().Bold(true)
	helpStyle          = lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1)
	statusBarStyle     = lipgloss.NewStyle().Foreground(colorMuted).Background(colorStatusBg)
	modalStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Background(colorModalBg).Foreground(colorModalFg).Padding(1, 2)
	modalTitleStyle    = lipgloss.NewStyle().Bold(true)
	modalKeyStyle      = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	modalHintStyle     = lipgloss.NewStyle().Foreground(colorMuted)
)

var parallelPrefixColors = []lipgloss.AdaptiveColor{
	{Light: "#1d4ed8", Dark: "#60a5fa"},
	{Light: "#9d174d", Dark: "#f472b6"},
	{Light: "#15803d", Dark: "#4ade80"},
	{Light: "#b45309", Dark: "#f59e0b"},
	{Light: "#0f766e", Dark: "#2dd4bf"},
	{Light: "#6d28d9", Dark: "#a78bfa"},
}

func parallelPrefixStyle(index int) lipgloss.Style {
	if len(parallelPrefixColors) == 0 {
		return lipgloss.NewStyle()
	}
	color := parallelPrefixColors[index%len(parallelPrefixColors)]
	return lipgloss.NewStyle().Foreground(color)
}

func statusStyle(status TaskStatus) lipgloss.Style {
	switch status {
	case StatusRunning:
		return lipgloss.NewStyle().Foreground(colorRunning)
	case StatusSuccess:
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case StatusFailed:
		return lipgloss.NewStyle().Foreground(colorFailed)
	case StatusCanceled:
		return lipgloss.NewStyle().Foreground(colorCanceled)
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}
