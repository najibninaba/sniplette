package progress

import "time"

// Stage identifies a high-level step in the pipeline.
type Stage string

const (
	StageDeps        Stage = "deps"
	StageMetadata    Stage = "metadata"
	StageDownloading Stage = "downloading"
	StageMerging     Stage = "merging"
	StageEncoding    Stage = "encoding"
	StageCompleted   Stage = "completed"
	StageError       Stage = "error"
)

// LogStream indicates which stream produced a log line.
type LogStream int

const (
	StreamStdout LogStream = iota
	StreamStderr
)

// Update conveys progress or stage changes for a job.
// Percent is 0..100 when known; set to a negative value (e.g., -1) to mean unknown.
type Update struct {
	JobID   string
	Stage   Stage
	Percent float64 // 0..100, or <0 if unknown

	ETA     *time.Duration // optional
	Bytes   *int64         // optional cumulative bytes
	Speed   *string        // optional, e.g., "2.5MiB/s" or "1.2x"
	Message string         // short human-friendly status line
}

// Log is a structured log line associated with a job.
type Log struct {
	JobID  string
	Stream LogStream
	Line   string
}

// Result is emitted once per job when it completes or fails.
type Result struct {
	JobID      string
	OutputPath string
	Bytes      int64
	Err        error // nil on success
}

// Reporter is implemented by UI or any observer interested in progress events.
type Reporter interface {
	Update(u Update)
	Log(l Log)
	Result(r Result)
}