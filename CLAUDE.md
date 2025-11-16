# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working with this Repository

This workspace is configured with RepoPrompt (RP) tools for enhanced development workflow:
- Use RP **chat tool** to plan and implement features to completion
- The chat agent can apply edits directly to files
- RP provides file operations, workspace management, and code structure analysis
- For complex changes, use the chat tool's planning mode first, then switch to implementation

### Git and GitHub Workflow

**Git Operations**: Use standard `git` commands via the Bash tool for all Git operations:
- Staging: `git add <files>`
- Committing: `git commit -m "message"`
- Pushing: `git push`
- Status: `git status`
- Diffing: `git diff`

**GitHub Operations**: Use the `gh` CLI tool for all GitHub-specific operations:
- Pull requests: `gh pr create`, `gh pr view`, `gh pr merge`
- Issues: `gh issue create`, `gh issue list`
- Repository operations: `gh repo view`, `gh repo fork`

**Conventional Commit Messages**: All commits must follow conventional commit format:
- Format: `type(scope): subject` or `type: subject`
- Common types:
  - `feat`: New feature
  - `fix`: Bug fix
  - `docs`: Documentation changes
  - `refactor`: Code refactoring
  - `test`: Test additions or changes
  - `chore`: Build process, dependencies, tooling
  - `perf`: Performance improvements
  - `style`: Code style/formatting (not CSS)
- Guidelines:
  - No emojis in commit messages
  - No self-attribution (e.g., no "Co-Authored-By: Claude")
  - Use imperative mood ("add" not "added" or "adds")
  - Keep subject line under 72 characters
  - Provide detailed body if needed, separated by blank line
- Examples:
  - `feat: add video resolution auto-detection`
  - `fix: handle empty metadata from yt-dlp`
  - `docs: update build instructions to use Make commands`
  - `refactor(encoder): extract bitrate calculation logic`

**Default Behavior**: After successfully implementing a task:
1. Review changes with `git status` and `git diff`
2. Stage modified/new files with `git add`
3. Create a conventional commit with clear, descriptive message
4. Push to remote with `git push` (unless explicitly told not to)

## Overview

Sniplette downloads videos from Instagram or YouTube and transcodes them into compact, messaging‑app‑friendly MP4 files using yt-dlp/youtube-dl and ffmpeg. It supports multiple URLs and provides a TUI for progress—perfect for producing tiny, snack-sized snips for sharing.

## Build & Development Commands

```bash
# Build the binary
go build ./cmd/sniplette

# Install to $GOBIN
go install ./cmd/sniplette

# Run directly
go run ./cmd/sniplette <instagram-or-youtube-url> [flags]

# Basic test run
./sniplette --dry-run https://www.instagram.com/reel/ABC123/
```

### Example

```bash
# Instagram
sniplette https://www.instagram.com/reel/ABC123/

# YouTube (short link)
sniplette https://youtu.be/XXXXXXXXXXX

# YouTube (full link)
sniplette https://www.youtube.com/watch?v=YYYYYYYYYYY
```

## Architecture

### High-Level Flow

1. **CLI Parsing** (`internal/cli/cmd/root.go`, `internal/cli/cmd/run.go`): Parses flags and validates URLs (Instagram: instagram.com, instagr.am; YouTube: youtube.com, youtu.be; Threads currently unsupported)
2. **Dependency Detection** (`internal/util/deps`): Locates `yt-dlp`/`youtube-dl` and `ffmpeg` in PATH
3. **UI Mode Selection** (`internal/cli/cmd/run.go:isTerminal`, `internal/ui/`): If stdout is a TTY and `--no-ui` not set → launches Bubble Tea TUI; otherwise → plain text mode
4. **Job Execution**:
   - **Download** (`internal/downloader/`): Fetches metadata and optionally downloads media via yt-dlp
   - **Encode** (`internal/encoder/`): Transcodes with ffmpeg using size-constrained bitrate or CRF mode
   - **Caption** (`internal/util/fs.go`): Writes caption text file alongside output

### Package Structure

- **`cmd/sniplette/main.go`**: Entry point
- **`internal/cli/`**: Cobra commands, flag parsing, URL validation, preset resolution
- **`internal/downloader/`**: Wraps yt-dlp/youtube-dl
- **`internal/encoder/`**: Wraps ffmpeg
- **`internal/ui/`**: Bubble Tea TUI (model, run, view, styles)
- **`internal/progress/`**: Progress reporting interface
- **`internal/model/`**: Core data structures
- **`internal/util/`**: Subprocess, filesystem, filename sanitization helpers

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

Presets are applied in `internal/cli/cmd/run.go` via `presetDefaults`:
- `low`: 540p, 20MB, CRF 26
- `medium` (default): 720p, 50MB, CRF 22
- `high`: 1080p, 100MB, CRF 19

Resolution and max-size can be overridden individually via `--resolution` and `--max-size-mb`.

### Shell Completion

- Bash:
  ```bash
  source <(sniplette completion bash)
  ```
- Zsh:
  ```bash
  sniplette completion zsh > "${fpath[1]}/_sniplette"
  ```
- Fish:
  ```bash
  sniplette completion fish | source
  ```
- PowerShell:
  ```powershell
  sniplette completion powershell | Out-String | Invoke-Expression
  ```

### Environment Variable

Primary env var: `SNIPLETTE_DL_BINARY` (with a compatibility fallback to `IG2WA_DL_BINARY` if the new var is unset).

### Filename Generation

Constructed in `buildOutputBasename()` functions (duplicated in `main.go` and `ui/model.go`):
- Format: `{uploader}_{id}_{resolution}p_{size_or_crf}.mp4`
- Example: `username_ABC123_720p_50MB.mp4` or `username_ABC123_1080p_CRF22.mp4`
- Uploader and ID are sanitized via `util.SanitizeFilename()` to remove unsafe characters

## Test Run

Try a few URLs to verify:

```bash
# Instagram
sniplette https://www.instagram.com/reel/ABC123/

# YouTube short
sniplette https://youtu.be/XXXXXXXXXXX

# YouTube full
sniplette https://www.youtube.com/watch?v=YYYYYYYYYYY
```