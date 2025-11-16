package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"ig2wa/internal/model"
)

// Run launches the TUI with the provided URLs and options.
// Integration with the main program and orchestration will be added next.
func Run(ctx context.Context, urls []string, opts model.CLIOptions) error {
	m := NewModel(ctx, urls, opts)
	prog := tea.NewProgram(m, tea.WithContext(ctx))
	final, err := prog.Run()
	if err != nil {
		return err
	}
	if fm, ok := final.(Model); ok {
		var failed []string
		for _, id := range fm.jobOrder {
			js := fm.jobs[id]
			if js != nil && js.err != nil {
				url := js.url
				msg := js.err.Error()
				if url != "" {
					failed = append(failed, fmt.Sprintf("- %s: %s", url, msg))
				} else {
					failed = append(failed, fmt.Sprintf("- %s", msg))
				}
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("%d job(s) failed:\n%s", len(failed), strings.Join(failed, "\n"))
		}
	}
	return nil
}