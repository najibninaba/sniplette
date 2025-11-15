# ig2wa

ig2wa is a small Go CLI tool that downloads Instagram videos/Reels and transcodes them into WhatsApp-friendly MP4 files. It orchestrates external tools (`yt-dlp` or `youtube-dl`, and `ffmpeg`) to fetch and encode content for sharing.

- Preferred downloader: `yt-dlp` (fallback to `youtube-dl`)
- Encoder: `ffmpeg`
- Output defaults: H.264/AAC in MP4, 720p long side, ≤50 MB target, `yuv420p` for compatibility
- Optional: audio-only extraction to `.m4a`
- Captions saved alongside videos by default (`.txt`)

This tool does not bypass authentication or DRM; it only works with publicly accessible URLs.

## Requirements

- Go 1.21+
- External tools in your `PATH`:
  - `yt-dlp` (preferred) or `youtube-dl`
  - `ffmpeg`

ig2wa detects tools at startup:
1. If `--dl-binary` is provided, it uses that path/name.
2. Else `IG2WA_DL_BINARY` env var.
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
- As a fallback, install `youtube-dl` if you can’t use `yt-dlp`.

## Build

From the repository root:

```bash
go build ./cmd/ig2wa
```

This produces a `ig2wa` binary (or `ig2wa.exe` on Windows) in the current directory.

To install into your `$GOBIN`:

```bash
go install ./cmd/ig2wa
```

## Usage

Basic syntax:

```bash
ig2wa <instagram-url> [<instagram-url> ...] [flags]
```

Core flags:
- `-o, --out-dir string` Output directory (default: `.`)
- `--max-size-mb int` Target max size per video in MB (default: 50; set 0 to use CRF/quality mode)
- `--quality-preset string` Preset quality: `low`, `medium`, `high` (default: `medium`)
- `--resolution int` Override long-side resolution in px (e.g., 540, 720, 1080)
- `--audio-only` Extract audio only (M4A)
- `--caption string` Caption output: `txt`, `none` (default: `txt`)
- `--keep-temp` Keep intermediate download files
- `--dl-binary string` Path or name for `yt-dlp`/`youtube-dl`
- `--dry-run` Show plan without executing
- `-v, --verbose` Show full subprocess commands/output

Quality presets mapping:
- `low`: 540p, max-size-mb=20, crf=26
- `medium` (default): 720p, max-size-mb=50, crf=22
- `high`: 1080p, max-size-mb=100, crf=19

Examples:

```bash
# Simple default (720p, ~50MB, caption saved)
ig2wa https://www.instagram.com/reel/ABC123/

# Multiple URLs
ig2wa https://www.instagram.com/reel/ABC123/ https://www.instagram.com/p/XYZ987/

# High quality, larger size
ig2wa --quality-preset high --max-size-mb 150 https://www.instagram.com/reel/ABC123/

# Force resolution override
ig2wa --resolution 1080 https://www.instagram.com/reel/ABC123/

# Audio-only extraction
ig2wa --audio-only https://www.instagram.com/reel/ABC123/

# Dry-run preview (shows plan and computed bitrate)
ig2wa --dry-run -v https://www.instagram.com/reel/ABC123/

# Use a custom downloader path/name
IG2WA_DL_BINARY=/usr/local/bin/yt-dlp ig2wa https://www.instagram.com/reel/ABC123/
# or
ig2wa --dl-binary /usr/local/bin/yt-dlp https://www.instagram.com/reel/ABC123/
```

## Output Details

- Video container: MP4
- Video codec: H.264 (`libx264`), `yuv420p` pixel format, `-preset veryfast`, profile `main`
- Audio codec: AAC at 96 kbps (configurable in code)
- Scaling:
  - Vertical (height > width): `scale=-2:LONG_SIDE`
  - Horizontal: `scale=LONG_SIDE:-2`
  - LONG_SIDE defaults to preset resolution (e.g., 720) or `--resolution`
- In size-constrained mode (default), video bitrate is computed from duration and `--max-size-mb`.
- In CRF mode (`--max-size-mb 0` or missing duration), use CRF from the preset (22 by default for `medium`).

Captions:
- By default, the original caption is written to a `.txt` file next to the video.
- Disable with `--caption none`.

## Exit Codes

- `0` success
- `1` invalid usage or CLI error
- `2` missing dependency (`yt-dlp`/`youtube-dl` or `ffmpeg`)
- `3` download error
- `4` transcode error

## Troubleshooting

- "Could not find yt-dlp or youtube-dl": Install `yt-dlp` and ensure it’s in `PATH`, or pass `--dl-binary`.
- "Could not find ffmpeg": Install `ffmpeg` and ensure it’s in `PATH`.
- Size slightly exceeds target: The bitrate calculation is approximate. Consider increasing `--max-size-mb`, lowering resolution, or switching to CRF mode.
- Non-ASCII titles/usernames: Filenames are sanitized and truncated to safe, UTF‑8‑preserving names.

## Build From Source (Recap)

```bash
# From repo root
go build ./cmd/ig2wa

# Run
./ig2wa --help
```

## Legal/Ethical Note

This tool should only be used to download videos you're allowed to (your own content, or with permission). You must comply with Instagram's Terms of Service and local copyright laws. This tool orchestrates yt-dlp/ffmpeg and does not bypass DRM or authentication — it only works with publicly accessible URLs.

## License

Copyright © 2025. See repository for license information (if/when added).