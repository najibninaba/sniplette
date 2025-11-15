package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"ig2wa/internal/model"
)

// Run launches the TUI with the provided URLs and options.
// Integration with the main program and orchestration will be added next.
func Run(ctx context.Context, urls []string, opts model.CLIOptions) error {
	m := NewModel(ctx, urls, opts)
	prog := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := prog.Run()
	return err
}