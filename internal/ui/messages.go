package ui

import "ig2wa/internal/progress"

type depsCheckedMsg struct {
	DownloaderPath string
	FFmpegPath     string
	Err            error
}

type jobStartMsg struct {
	JobID string
}

type jobUpdateMsg struct {
	U progress.Update
}

type jobLogMsg struct {
	L progress.Log
}

type jobResultMsg struct {
	R progress.Result
}

type allDoneMsg struct{}