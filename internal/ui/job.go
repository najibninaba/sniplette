package ui

import (
	bubblesprogress "github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"ig2wa/internal/progress"
)

type jobState struct {
	id     string
	url    string
	stage  progress.Stage
	status string
	err    error
	done   bool

	outputPath string
	bytes      int64
	percent    float64 // -1 means unknown

	spinner spinner.Model
	bar     bubblesprogress.Model

	started bool

	// Optional: recent logs (kept small)
	logsRing []string
}

func newJobState(id, url string, styles Styles) jobState {
	sp := spinner.New()
	sp.Style = styles.Spinner
	bar := bubblesprogress.New(
		bubblesprogress.WithDefaultGradient(),
		bubblesprogress.WithWidth(40),
	)
	return jobState{
		id:      id,
		url:     url,
		stage:   progress.StageMetadata,
		status:  "Queued",
		percent: -1,
		spinner: sp,
		bar:     bar,
	}
}