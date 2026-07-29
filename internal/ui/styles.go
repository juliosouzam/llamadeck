package ui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.AdaptiveColor{Light: "#6C3FD1", Dark: "#B69CFF"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B8B8B"}
	colSubtle = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#5F5F5F"}
	colOK     = lipgloss.AdaptiveColor{Light: "#0B7A3B", Dark: "#5BD68A"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	colErr    = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF7B72"}
	colValue  = lipgloss.AdaptiveColor{Light: "#0F6FA8", Dark: "#79C0FF"}

	styTitle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styMuted = lipgloss.NewStyle().Foreground(colMuted)
	stySub   = lipgloss.NewStyle().Foreground(colSubtle)
	styOK    = lipgloss.NewStyle().Foreground(colOK).Bold(true)
	styWarn  = lipgloss.NewStyle().Foreground(colWarn).Bold(true)
	styErr   = lipgloss.NewStyle().Foreground(colErr).Bold(true)
	styValue = lipgloss.NewStyle().Foreground(colValue)
	styGroup = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styOn    = lipgloss.NewStyle().Foreground(colOK)

	styTabActive   = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Underline(true)
	styTabInactive = lipgloss.NewStyle().Foreground(colMuted)

	styCursor = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styRule   = lipgloss.NewStyle().Foreground(colSubtle)
	styBadge  = lipgloss.NewStyle().Foreground(colWarn)
	styToast  = lipgloss.NewStyle().Foreground(colOK)
)

func rule(w int) string {
	if w <= 0 {
		return ""
	}
	b := make([]rune, w)
	for i := range b {
		b[i] = '─'
	}
	return styRule.Render(string(b))
}
