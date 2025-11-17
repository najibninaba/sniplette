package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	bubblesprogress "github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"ig2wa/internal/downloader"
	"ig2wa/internal/encoder"
	"ig2wa/internal/model"
	"ig2wa/internal/progress"
	"ig2wa/internal/util"
	"ig2wa/internal/util/deps"
	"ig2wa/internal/util/media"
	"ig2wa/internal/pipeline"
	"ig2wa/internal/util/format"
)

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	// App state (deps)
	depsChecked    bool
	depsErr        error
	downloaderPath string
	ffmpegPath     string

	// Jobs
	urls     []string
	opts     model.CLIOptions
	jobOrder []string
	jobs     map[string]*jobState
	selected int
	workers  int
	running  int
	next     int // next index in urls to start

	// UI
	width, height int
	styles        Styles

	// Internal event channel used by reporter to feed tea messages
	eventCh chan tea.Msg
}

func NewModel(ctx context.Context, urls []string, opts model.CLIOptions) Model {
	c, cancel := context.WithCancel(ctx)
	sty := defaultStyles()

	jobs := make(map[string]*jobState, len(urls))
	order := make([]string, 0, len(urls))
	for i, u := range urls {
		id := toID(i, u)
		js := newJobState(id, u, sty)
		js.bar = bubblesprogress.New(bubblesprogress.WithDefaultGradient(), bubblesprogress.WithWidth(40))
		jobs[id] = &js
		order = append(order, id)
	}

	workers := opts.Jobs
	if workers <= 0 {
		workers = 2
	}

	return Model{
		ctx:      c,
		cancel:   cancel,
		urls:     urls,
		opts:     opts,
		jobs:     jobs,
		jobOrder: order,
		selected: 0,
		workers:  workers,
		styles:   sty,
		eventCh:  make(chan tea.Msg, 256),
	}
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, id := range m.jobOrder {
		sp := m.jobs[id].spinner
		cmds = append(cmds, sp.Tick)
	}
	// Listen for reporter events
	cmds = append(cmds, m.listenEventsCmd())
	// Kick off dependency check
	cmds = append(cmds, m.checkDepsCmd())
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case depsCheckedMsg:
		m.depsChecked = true
		m.depsErr = msg.Err
		m.downloaderPath = msg.DownloaderPath
		m.ffmpegPath = msg.FFmpegPath
		if m.depsErr != nil {
			// Mark all as errored
			for _, id := range m.jobOrder {
				js := m.jobs[id]
				js.stage = progress.StageError
				js.status = fmt.Sprintf("Dependency error: %v", m.depsErr)
				js.err = m.depsErr
				js.done = true
			}
			return m, tea.Quit
		}
		// Start initial workers
		return m, m.startNextWorkersCmd()

	case jobUpdateMsg:
		u := msg.U
		if js, ok := m.jobs[u.JobID]; ok {
			js.stage = u.Stage
			js.percent = u.Percent
			js.status = u.Message
			if u.Bytes != nil {
				js.bytes = *u.Bytes
			}
		}
	case jobLogMsg:
		l := msg.L
		if js, ok := m.jobs[l.JobID]; ok {
			// small ring buffer
			line := strings.TrimRight(l.Line, "\r\n")
			if len(js.logsRing) > 1000 {
				js.logsRing = js.logsRing[1:]
			}
			js.logsRing = append(js.logsRing, line)
		}
	case jobResultMsg:
		r := msg.R
		if js, ok := m.jobs[r.JobID]; ok {
			js.done = true
			js.err = r.Err
			if r.Err == nil {
				js.stage = progress.StageCompleted
				js.percent = 100
				js.outputPath = r.OutputPath
				js.bytes = r.Bytes
				// Set informative status with basename and size
				if r.OutputPath != "" {
					name := filepath.Base(r.OutputPath)
					size := format.HumanizeBytes(r.Bytes)
					if m.opts.DryRun {
						js.status = fmt.Sprintf("Planned: %s (%s)", name, size)
					} else {
						js.status = fmt.Sprintf("Saved: %s (%s)", name, size)
					}
				} else {
					js.status = "Completed"
				}
			} else {
				js.stage = progress.StageError
				js.status = r.Err.Error()
				js.percent = -1
			}
			m.running--
			// Start next job if any remain
			return m, m.startNextWorkersCmd()
		}
	case allDoneMsg:
		return m, tea.Quit
	}

	// Update per-job components (spinner)
	var cmds []tea.Cmd
	for _, id := range m.jobOrder {
		js := m.jobs[id]
		var c tea.Cmd
		js.spinner, c = js.spinner.Update(msg)
		if c != nil {
			cmds = append(cmds, c)
		}
	}
	// Keep listening for events
	cmds = append(cmds, m.listenEventsCmd())
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	summary := m.viewSummary()
	if summary != "" {
		return m.viewHeader() + "\n\n" + m.viewJobs() + "\n" + summary
	}
	return m.viewHeader() + "\n\n" + m.viewJobs()
}

func (m Model) listenEventsCmd() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
			return allDoneMsg{}
		case msg := <-m.eventCh:
			return msg
		}
	}
}

func (m Model) checkDepsCmd() tea.Cmd {
	return func() tea.Msg {
		dl, derr := deps.FindDownloader(m.opts.DLBinary)
		if derr != nil {
			return depsCheckedMsg{Err: derr}
		}
		ff, ferr := deps.FindFFmpeg()
		if ferr != nil {
			return depsCheckedMsg{Err: ferr}
		}
		return depsCheckedMsg{DownloaderPath: dl, FFmpegPath: ff, Err: nil}
	}
}

func (m Model) startNextWorkersCmd() tea.Cmd {
	return func() tea.Msg {
		// If canceled, stop
		select {
		case <-m.ctx.Done():
			return allDoneMsg{}
		default:
		}
		for m.running < m.workers && m.next < len(m.urls) {
			idx := m.next
			jobID := m.jobOrder[idx]
			url := m.urls[idx]
			m.next++
			m.running++
			// Mark job started
			if js := m.jobs[jobID]; js != nil {
				js.started = true
				js.status = "Queued"
				js.stage = progress.StageMetadata
			}
			// Launch job goroutine
			go m.runJob(jobID, url)
		}
		if m.next >= len(m.urls) && m.running == 0 {
			return allDoneMsg{}
		}
		// No specific message now; rely on reporter events
		return nil
	}
}

func (m Model) runJob(jobID, url string) {
	rep := teaReporter{ch: m.eventCh}

	// Step 1: Download metadata (or full if not dry-run)
	dv, tempDir, derr := downloader.Download(m.ctx, url, downloader.Options{
		DownloaderPath: m.downloaderPath,
		Verbose:        m.opts.Verbose,
		KeepTemp:       m.opts.KeepTemp,
		MetadataOnly:   m.opts.DryRun,
		Reporter:       rep,
		JobID:          jobID,
	})
	// Cleanup unless keep-temp
	defer func() {
		if !m.opts.KeepTemp && tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if derr != nil {
		rep.Result(progress.Result{JobID: jobID, Err: fmt.Errorf("downloader: %w", derr)})
		return
	}

	// Plan encoding
	targetLongSide, usedCRF := pipeline.PlanResolutionAndCRF(m.opts, dv, pipeline.DefaultCRF(m.opts.Quality))
	encOpts := model.EncodeOptions{
		LongSidePx:       targetLongSide,
		ModeCRF:          m.opts.MaxSizeMB == 0 || dv.DurationSec <= 0 || m.opts.AudioOnly,
		CRF:              usedCRF,
		MaxSizeMB:        m.opts.MaxSizeMB,
		AudioBitrateKbps: 96,
		VideoMinKbps:     500,
		VideoMaxKbps:     8000,
		Preset:           "veryfast",
		Profile:          "main",
		AudioOnly:        m.opts.AudioOnly,
		KeyInt:           48,
	}

	// Dry run: no encode, just finalize result
	ext := ".mp4"
	if m.opts.AudioOnly {
		ext = ".m4a"
	}
	base := media.OutputBasename(dv, targetLongSide, m.opts.MaxSizeMB, encOpts)
	outputPath := filepath.Join(m.opts.OutDir, base+ext)

	if m.opts.DryRun {
		// Present plan as status
		name := filepath.Base(outputPath)
		rep.Update(progress.Update{
			JobID:   jobID,
			Stage:   progress.StageCompleted,
			Percent: 100,
			Message: fmt.Sprintf("Planned: %s (dry-run)", name),
		})
		rep.Result(progress.Result{JobID: jobID, OutputPath: outputPath, Bytes: 0, Err: nil})
		return
	}

	// Encode
	out, eerr := encoder.Encode(m.ctx, dv, encOpts, encoder.Options{
		FFmpegPath: m.ffmpegPath,
		Verbose:    m.opts.Verbose,
		OutputPath: outputPath,
		Reporter:   rep,
		JobID:      jobID,
	})
	if eerr != nil {
		rep.Result(progress.Result{JobID: jobID, Err: fmt.Errorf("encode: %w", eerr)})
		return
	}

	// Caption
	if m.opts.Caption == model.CaptionTxt {
		caption := media.CaptionText(dv)
		if _, werr := util.WriteCaptionFile(out.OutputPath, caption); werr != nil && m.opts.Verbose {
			rep.Log(progress.Log{JobID: jobID, Stream: progress.StreamStderr, Line: fmt.Sprintf("warning: failed to write caption: %v", werr)})
		}
	}

	// Send final update with filename before result
	name := filepath.Base(out.OutputPath)
	size := format.HumanizeBytes(out.Bytes)
	rep.Update(progress.Update{
		JobID:   jobID,
		Stage:   progress.StageCompleted,
		Percent: 100,
		Message: fmt.Sprintf("Saved: %s (%s)", name, size),
	})

	rep.Result(progress.Result{JobID: jobID, OutputPath: out.OutputPath, Bytes: out.Bytes, Err: nil})
}

type teaReporter struct {
	ch chan tea.Msg
}

func (r teaReporter) Update(u progress.Update) {
	// Block on completion messages to ensure they're delivered
	if u.Stage == progress.StageCompleted || u.Stage == progress.StageError {
		r.ch <- jobUpdateMsg{U: u}
		return
	}
	select {
	case r.ch <- jobUpdateMsg{U: u}:
	default:
	}
}
func (r teaReporter) Log(l progress.Log) {
	select {
	case r.ch <- jobLogMsg{L: l}:
	default:
	}
}
func (r teaReporter) Result(res progress.Result) {
	// Always block on Result messages - they're critical
	r.ch <- jobResultMsg{R: res}
}

func findDownloader(custom string) (string, error) {
	if custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom, nil
		}
		if p, err := exec.LookPath(custom); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("could not find downloader at %q", custom)
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("youtube-dl"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find yt-dlp or youtube-dl in PATH. Please install yt-dlp.")
}

func findFFmpeg() (string, error) {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find ffmpeg in PATH. Please install ffmpeg.")
}

func planResolutionAndCRF(opts model.CLIOptions, dv model.DownloadedVideo) (int, int) {
	target := opts.Resolution
	if target <= 0 {
		target = 720
	}
	inLong := max(dv.Width, dv.Height)
	if inLong > 0 && inLong < target {
		target = inLong
	}
	return target, defaultCRF(opts.Quality)
}

func defaultCRF(q model.QualityPreset) int {
	switch q {
	case model.PresetLow:
		return 26
	case model.PresetHigh:
		return 19
	case model.PresetMedium:
		fallthrough
	default:
		return 22
	}
}

func buildOutputBasename(dv model.DownloadedVideo, longSide int, maxSizeMB int, enc model.EncodeOptions) string {
	uploader := dv.Uploader
	if uploader == "" {
		uploader = "ig"
	}
	id := dv.ID
	if id == "" {
		id = dv.Title
	}
	uploader = util.SanitizeFilename(uploader)
	id = util.SanitizeFilename(id)

	parts := []string{uploader, id}
	if enc.AudioOnly {
		parts = append(parts, "audio")
	} else {
		parts = append(parts, fmt.Sprintf("%dp", longSide))
		if enc.ModeCRF {
			parts = append(parts, fmt.Sprintf("CRF%d", enc.CRF))
		} else if maxSizeMB > 0 {
			parts = append(parts, fmt.Sprintf("%dMB", maxSizeMB))
		}
	}
	return strings.Join(parts, "_")
}

func buildCaptionText(dv model.DownloadedVideo) string {
	var b strings.Builder
	title := strings.TrimSpace(dv.Title)
	uploader := strings.TrimSpace(dv.Uploader)
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	}
	if uploader != "" {
		b.WriteString(uploader)
		b.WriteString("\n")
	}
	if dv.URL != "" {
		b.WriteString(dv.URL)
		b.WriteString("\n")
	}
	b.WriteString("\n---\nORIGINAL CAPTION\n")
	if dv.Description != "" {
		b.WriteString(dv.Description)
		b.WriteString("\n")
	}
	return b.String()
}

func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func toID(i int, _ string) string {
	return "job-" + strconv.Itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	sign := ""
	if i < 0 {
		sign = "-"
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + (i % 10))
		i /= 10
	}
	if sign != "" {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}