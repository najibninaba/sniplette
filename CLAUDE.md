# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working with this Repository

This workspace is configured with RepoPrompt (RP) tools for enhanced development workflow:
- Use RP **chat tool** to plan and implement features to completion
- The chat agent can apply edits directly to files
- RP provides file operations, workspace management, and code structure analysis
- For complex changes, use the chat tool's planning mode first, then switch to implementation

## Overview

ig2wa is a Go CLI tool that downloads Instagram videos/Reels and transcodes them into WhatsApp-friendly MP4 files. It orchestrates `yt-dlp` (or `youtube-dl`) for downloading and `ffmpeg` for encoding.

## Build & Development Commands

```bash
# Build the binary
go build ./cmd/ig2wa

# Install to $GOBIN
go install ./cmd/ig2wa

# Run directly
go run ./cmd/ig2wa <instagram-url> [flags]

# Basic test run
./ig2wa --dry-run https://www.instagram.com/reel/ABC123/
```

## Architecture

### High-Level Flow

1. **CLI Parsing** (`internal/cli/flags.go`): Parses command-line flags and validates Instagram URLs
2. **Dependency Detection** (`cmd/ig2wa/main.go`): Locates `yt-dlp`/`youtube-dl` and `ffmpeg` in PATH
3. **UI Mode Selection** (`cmd/ig2wa/main.go:40-52`):
   - If stdout is a TTY and `--no-ui` not set → launches Bubble Tea TUI (`internal/ui/`)
   - Otherwise → falls back to plain text output
4. **Job Execution**:
   - **Download** (`internal/downloader/`): Fetches metadata and optionally downloads media via yt-dlp
   - **Encode** (`internal/encoder/`): Transcodes with ffmpeg using size-constrained bitrate or CRF mode
   - **Caption** (`internal/util/fs.go`): Writes caption text file alongside output

### Package Structure

- **`cmd/ig2wa/main.go`**: Entry point, dependency detection, orchestration of legacy text mode
- **`internal/cli/`**: Flag parsing, URL validation, preset resolution
- **`internal/downloader/`**: Wraps yt-dlp/youtube-dl; parses JSON metadata; handles progress parsing from download output
- **`internal/encoder/`**: Wraps ffmpeg; computes bitrate from target size or uses CRF; parses ffmpeg's `-progress` output
- **`internal/ui/`**: Bubble Tea TUI implementation
  - `model.go`: TUI state machine, job orchestration, worker pool (default 2 concurrent jobs)
  - `run.go`: Entry point for TUI mode
  - `job.go`, `messages.go`, `view.go`, `styles.go`: UI rendering and message handling
- **`internal/progress/`**: Shared progress reporting interface used by downloader/encoder to feed TUI
- **`internal/model/`**: Core data structures (CLIOptions, DownloadedVideo, EncodeOptions, OutputVideo)
- **`internal/util/`**: Helper utilities for running subprocesses, filesystem operations, filename sanitization

### Encoding Modes

The tool supports two encoding strategies, selected automatically:

1. **Size-Constrained Mode** (default):
   - Computes video bitrate from `--max-size-mb`, duration, and audio bitrate
   - Used when `--max-size-mb > 0` and duration is known
   - Bitrate calculation: `internal/encoder/encoder.go:193-208`

2. **CRF Mode** (quality-based):
   - Uses constant rate factor for quality control
   - Activated when `--max-size-mb 0` or duration unknown or `--audio-only`
   - CRF values: low=26, medium=22, high=19

### Progress Reporting

The codebase uses a `progress.Reporter` interface (`internal/progress/progress.go`) to decouple progress tracking from UI:
- Downloader parses yt-dlp's `[download]` lines to extract percent, speed, ETA (`internal/downloader/downloader.go:229-283`)
- Encoder parses ffmpeg's `-progress pipe:1` key=value format (`internal/encoder/encoder.go:108-157`)
- TUI mode uses `teaReporter` (`internal/ui/model.go:353-377`) to forward events to the Bubble Tea event loop via channels

### Exit Codes

- `0`: Success
- `1`: CLI error or invalid usage
- `2`: Missing dependency (yt-dlp/youtube-dl or ffmpeg)
- `3`: Download error
- `4`: Transcode error

## Important Implementation Details

### Downloader (`internal/downloader/downloader.go`)

- Always fetches metadata first via `--dump-json` to get duration, dimensions, title
- If `MetadataOnly` is true (dry-run), returns early without downloading media
- Downloads to temp directory with template `%(id)s.%(ext)s` for predictable file resolution
- Prefers `.mp4` extension when multiple files found (see `extPriority` function)
- Caller is responsible for cleaning up temp directory unless `--keep-temp` is set

### Encoder (`internal/encoder/encoder.go`)

- Audio-only mode (`--audio-only`) extracts AAC audio to `.m4a` with no video stream
- Video encoding uses H.264 (`libx264`), AAC audio, `yuv420p` pixel format for WhatsApp compatibility
- Scale filter logic in `scaleFilter()`: vertical videos use `scale=-2:HEIGHT`, horizontal use `scale=WIDTH:-2`
- Always sets `-movflags +faststart` for web streaming
- In TUI mode with verbose off, uses `-progress pipe:1 -nostats` to get machine-readable progress on stdout

### TUI Implementation (`internal/ui/`)

- Built with Bubble Tea (charmbracelet/bubbletea)
- Worker pool pattern: launches up to `--jobs` concurrent goroutines (default 2)
- Each job runs in a goroutine that communicates via `eventCh` channel
- Events are converted to Bubble Tea messages: `jobUpdateMsg`, `jobLogMsg`, `jobResultMsg`
- Job state tracked in `jobState` struct with stage, percent, status, error, logs ring buffer
- Spinners from charmbracelet/bubbles used for visual feedback

### Quality Presets

Defined in `internal/cli/flags.go:148-159`:
- `low`: 540p, 20MB, CRF 26
- `medium` (default): 720p, 50MB, CRF 22
- `high`: 1080p, 100MB, CRF 19

Resolution and max-size can be overridden individually via `--resolution` and `--max-size-mb`.

### Filename Generation

Constructed in `buildOutputBasename()` functions (duplicated in `main.go` and `ui/model.go`):
- Format: `{uploader}_{id}_{resolution}p_{size_or_crf}.mp4`
- Example: `username_ABC123_720p_50MB.mp4` or `username_ABC123_1080p_CRF22.mp4`
- Uploader and ID are sanitized via `util.SanitizeFilename()` to remove unsafe characters