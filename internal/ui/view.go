package ui

import (
	"fmt"
	"strings"

	"ig2wa/internal/progress"
)

func (m Model) viewHeader() string {
	done, total := 0, len(m.jobOrder)
	for _, id := range m.jobOrder {
		if m.jobs[id].done {
			done++
		}
	}
	title := m.styles.Title.Render("ig2wa — Instagram to WhatsApp")
	sub := m.styles.Subtitle.Render(fmt.Sprintf("Jobs: %d/%d done • q: quit", done, total))
	return title + "\n" + sub
}

func (m Model) viewJobs() string {
	var b strings.Builder
	for _, id := range m.jobOrder {
		js := m.jobs[id]
		b.WriteString(m.viewJob(js))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewJob(js *jobState) string {
	stageStyle := m.styles.JobInfo
	switch js.stage {
	case progress.StageMetadata:
		stageStyle = m.styles.StageMeta
	case progress.StageDownloading, progress.StageMerging:
		stageStyle = m.styles.StageDL
	case progress.StageEncoding:
		stageStyle = m.styles.StageEnc
	case progress.StageCompleted:
		stageStyle = m.styles.Success
	case progress.StageError:
		stageStyle = m.styles.Error
	}

	left := m.styles.JobTitle.Render(truncate(js.url, 48))
	stage := stageStyle.Render(string(js.stage))

	var right string
	if js.percent >= 0 && js.percent <= 100 {
		right = fmt.Sprintf("%s %5.1f%%", js.bar.ViewAs(js.percent/100.0), js.percent)
	} else if js.done && js.err == nil {
		right = m.styles.Success.Render("✓ done")
	} else if js.err != nil {
		right = m.styles.Error.Render("✗ error")
	} else {
		right = m.styles.Spinner.Render(js.spinner.View()) + " " + m.styles.Faint.Render("waiting")
	}

	info := js.status
	line1 := fmt.Sprintf("%s  %s", left, stage)
	line2 := m.styles.JobInfo.Render(info)
	return m.styles.Box.Render(line1+"\n"+right+"\n"+line2)
}

func (m Model) viewSummary() string {
	var completed []string
	for _, id := range m.jobOrder {
		js := m.jobs[id]
		if js.done && js.err == nil && js.outputPath != "" {
			completed = append(completed, js.outputPath)
		}
	}
	
	if len(completed) == 0 {
		return ""
	}
	
	var b strings.Builder
	b.WriteString(m.styles.Subtitle.Render("✓ Completed Files:"))
	b.WriteString("\n")
	for _, path := range completed {
		b.WriteString(m.styles.Success.Render("  • " + path))
		b.WriteString("\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	if n <= 0 || len([]rune(s)) <= n {
		return s
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}