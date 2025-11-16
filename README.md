# Sniplette

Sniplette is a tiny video helper that turns large Instagram and YouTube videos into small, shareable clips. Give it a link, and Sniplette will fetch → transcode → compress → and hand you a neat little "snip" perfect for messaging apps, chats, and social platforms.

### ✨ Features
- 📥 Downloads from Instagram and YouTube using `yt-dlp`
- 🎞️ Re-encodes with `ffmpeg` for consistent, mobile-friendly formats
- 📦 Shrinks videos down to configurable size limits (e.g., 50 MB)
- 📱 Ensures compatibility with messaging apps like WhatsApp, Telegram, and iMessage
- 🖼️ Outputs clean MP4/H.264 (yuv420p) or lightweight audio/video variants
- 🛠️ Simple, compact Go CLI

This tool does not bypass authentication or DRM; it only works with publicly accessible URLs.

## Supported platforms
- Instagram: `instagram.com`, `instagr.am`
- YouTube: `youtube.com`, `youtu.be`

## Requirements

- Go 1.21+
- External tools in your `PATH`:
  - `yt-dlp` (preferred) or `youtube-dl`
  - `ffmpeg`

Sniplette detects tools at startup:
1. If `--dl-binary` is provided, it uses that path/name.
2. Else `SNIPLETTE_DL_BINARY` env var (with compatibility fallback to `IG2WA_DL_BINARY`).
3. Else search `yt-dlp`, then `youtube-dl`.
4. It must also find `ffmpeg`, otherwise it exits with a helpful message.

## Install Dependencies

Below are common ways to install `yt-dlp` and `ffmpeg`. Choose what fits your system best.

### macOS (Homebrew)

```bash
brew install yt-dlp ffmpeg
```

### Ubuntu/Debian

```bash
# ffmpeg from apt
sudo apt-get update
sudo apt-get install -y ffmpeg

# yt-dlp via pip (recommended)
python3 -m pip install --user -U yt-dlp
# Make sure pip's bin dir is in your PATH (often ~/.local/bin)
```

Alternatively (if your distro provides it):
```bash
sudo apt-get install -y yt-dlp
```

### Arch Linux

```bash
sudo pacman -S yt-dlp ffmpeg
```

### Other Distros

- Install `ffmpeg` with your package manager.
- Install `yt-dlp` with your package manager or via `pip install -U yt-dlp`.
- As a fallback, install `youtube-dl` if you can't use `yt-dlp`.

## Build

From the repository root:

```bash
go build ./cmd/sniplette
```

This produces a `sniplette` binary (or `sniplette.exe` on Windows) in the current directory.

To install into your `$GOBIN`:

```bash
go install ./cmd/sniplette
```

## Usage

Sniplette uses the Cobra framework with subcommands. You can run directly with a URL or use explicit subcommands:

```bash
# Direct run (no subcommand)
sniplette <url> [<url> ...] [flags]

# Run download/encode pipeline
sniplette run <url> [<url> ...] [flags]

# Show plan (metadata-only) without executing
sniplette plan <url> [<url> ...] [flags]

# Force TUI mode for interactive runs
sniplette tui <url> [<url> ...] [flags]

# Diagnose external dependencies
sniplette doctor

# Generate shell completion scripts
sniplette completion [bash|zsh|fish|powershell]
```

## Commands

- run
  - Description: Execute the fetch/transcode pipeline for tiny, snack-sized snips.
  - Usage: `sniplette run [urls...] [flags]`

- plan
  - Description: Show a tiny plan (metadata-only) without running encoder or writing outputs.
  - Usage: `sniplette plan [urls...] [flags]`

- tui
  - Description: Force TUI mode for interactive snips (jobs, progress, etc.).
  - Usage: `sniplette tui [urls...] [flags]`
  - Notes: If stdout is not a terminal, this will error appropriately.

- doctor
  - Description: Diagnose external tools and show resolved paths.
  - Usage: `sniplette doctor`
  - Output example:
    ```
    Downloader: /usr/local/bin/yt-dlp
    FFmpeg:    /opt/homebrew/bin/ffmpeg
    ```

- completion
  - Description: Generate shell completion scripts.
  - Usage: `sniplette completion [bash|zsh|fish|powershell]`

## Flags

Core flags (available for subcommands):

- `-o, --out-dir string` Output directory (default: `.`)
- `--max-size-mb int` Target max size per video in MB (default: 50; set 0 to use CRF/quality mode)
- `--quality-preset string` Preset quality: `low`, `medium`, `high` (default: `medium`)
- `--resolution int` Override long-side resolution in px (e.g., 540, 720, 1080)
- `--audio-only` Extract audio only (M4A)
- `--caption string` Caption output: `txt`, `none` (default: `txt`)
- `--keep-temp` Keep intermediate download files
- `--dl-binary string` Path or name for `yt-dlp`/`youtube-dl`
- `-v, --verbose` Show full subprocess commands/output
- `--jobs int` Max concurrent jobs in TUI (default: 2)

Quality presets mapping:
- `low`: 540p, max-size-mb=20, crf=26
- `medium` (default): 720p, max-size-mb=50, crf=22
- `high`: 1080p, max-size-mb=100, crf=19

## Examples

Subcommand syntax:

```bash
# Explicit run
sniplette run https://www.instagram.com/reel/ABC123/

# Plan
sniplette plan -v https://www.instagram.com/reel/ABC123/

# Force TUI
sniplette tui --jobs 4 https://www.instagram.com/reel/ABC123/ https://youtu.be/BBB

# Dependency check
sniplette doctor

# Generate shell completion for zsh (see below to load)
sniplette completion zsh
```

## Shell Completion

Load completions for your current shell:

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

You can persist these according to your shell's standard initialization configuration.

## Output Details

- Video container: MP4
- Video codec: H.264 (`libx264`), `yuv420p` pixel format, `-preset veryfast`, profile `main`
- Audio codec: AAC at 96 kbps (configurable in code)
- Scaling:
  - Vertical (height > width): `scale=-2:LONG_SIDE`
  - Horizontal: `scale=LONG_SIDE:-2`
- Size/quality modes:
  - Size-constrained (default): Computes bitrate from duration and `--max-size-mb` for compact results.
  - CRF mode: Use `--max-size-mb 0` to switch to quality-based CRF encoding (preset CRFs: low=26, medium=22, high=19).

Captions:
- By default, the original caption is written to a `.txt` file next to the snip.
- Disable with `--caption none`.

## Exit Codes

- `0` success
- `1` invalid usage or CLI error
- `2` missing dependency (`yt-dlp`/`youtube-dl` or `ffmpeg`)
- `3` download error
- `4` transcode error

## Threads Support

Threads URLs are currently not supported.

Sniplette relies on yt-dlp for metadata and media extraction. yt-dlp does not have a Threads (threads.net) extractor as of now, so attempts to download Threads posts fail. Sniplette detects Threads URLs and fails fast with a clear error instead of attempting a broken download.

- Upstream issue: https://github.com/yt-dlp/yt-dlp/issues/7523
- Workaround: Use Instagram or YouTube URLs.
- Future: We may add an experimental native Threads extractor in the tool if there is sufficient demand.

## Troubleshooting

- "Could not find yt-dlp or youtube-dl": Install `yt-dlp` and ensure it's in `PATH`, or pass `--dl-binary`.
- "Could not find ffmpeg": Install `ffmpeg` and ensure it's in `PATH`.
- Size slightly exceeds target: The bitrate calculation is approximate. Consider increasing `--max-size-mb`, lowering resolution, or switching to CRF mode.
- Non-ASCII titles/usernames: Filenames are sanitized and truncated to safe, UTF‑8‑preserving names.

## Build From Source (Recap)

```bash
# From repo root
go build ./cmd/sniplette

# Run
./sniplette --help
```

## Legal/Ethical Note

This tool should only be used to download videos you're allowed to (your own content, or with permission). You must comply with Instagram's Terms of Service and local copyright laws. This tool orchestrates yt-dlp/ffmpeg and does not bypass DRM or authentication — it only works with publicly accessible URLs.

## License

Copyright © 2025. See repository for license information (if/when added).