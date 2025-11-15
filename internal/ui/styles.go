package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Header    lipgloss.Style
	JobTitle  lipgloss.Style
	JobInfo   lipgloss.Style
	Success   lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Faint     lipgloss.Style
	Box       lipgloss.Style
	Spinner   lipgloss.Style
	StageMeta lipgloss.Style
	StageDL   lipgloss.Style
	StageEnc  lipgloss.Style
}

func defaultStyles() Styles {
	base := lipgloss.NewStyle()
	return Styles{
		Title:     base.Bold(true).Foreground(lipgloss.Color("#7D56F4")),
		Subtitle:  base.Faint(true),
		Header:    base.Bold(true),
		JobTitle:  base.Foreground(lipgloss.Color("#A3A3A3")),
		JobInfo:   base.Foreground(lipgloss.Color("#D1D5DB")),
		Success:   base.Foreground(lipgloss.Color("#22C55E")),
		Error:     base.Foreground(lipgloss.Color("#EF4444")),
		Warning:   base.Foreground(lipgloss.Color("#F59E0B")),
		Faint:     base.Faint(true),
		Box:       base.Padding(0, 1),
		Spinner:   base.Foreground(lipgloss.Color("#22D3EE")),
		StageMeta: base.Foreground(lipgloss.Color("#60A5FA")),
		StageDL:   base.Foreground(lipgloss.Color("#06B6D4")),
		StageEnc:  base.Foreground(lipgloss.Color("#D946EF")),
	}
}