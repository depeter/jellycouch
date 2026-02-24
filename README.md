# JellyCouch

A lightweight, keyboard-first 10-foot Jellyfin client built with Go, Ebitengine, and libmpv.

## Why?

The existing Jellyfin Desktop client wraps a full Chromium instance — it's slow and memory-hungry. Kodi has the right UX but suffers from instability. JellyCouch aims to be a fast, couch-optimized alternative with fluid playback and proper subtitle controls.

## Architecture

**Two-mode rendering in a single window:**

- **Browse Mode** — Ebitengine renders poster grids, metadata, search UI at 60fps. Arrow keys navigate, Enter selects.
- **Play Mode** — libmpv takes over the window surface via `--wid`. Zero frame copying, zero GC pressure. Go only routes keyboard input to mpv commands.

## Features

- Arrow-key / gamepad navigation (10-foot UI)
- Poster grid browsing with async image loading and disk cache
- Library, search, and item detail screens
- Season/episode browsing for TV shows
- libmpv video playback with hardware acceleration
- Subtitle configuration (font, size, color, border, position, delay)
- Playback progress reporting and resume
- Mark watched/unwatched
- TOML configuration (`~/.config/jellycouch/config.toml`)

## Dependencies

### Build

- Go 1.24+
- `libmpv-dev`
- `pkg-config`
- `libx11-dev`
- `libgl-dev`

On Debian/Ubuntu:

```bash
sudo apt install libmpv-dev pkg-config libx11-dev libgl-dev
```

### Runtime

- `libmpv` (usually installed as part of mpv)
- X11 or XWayland

## Build

```bash
go build -o jellycouch ./cmd/jellycouch
```

## Run

```bash
./jellycouch
```

On first launch, you'll see a login screen to connect to your Jellyfin server.

## Configuration

Config is stored at `~/.config/jellycouch/config.toml`. Example:

```toml
[server]
url = "https://jellyfin.example.com"
username = "user"

[subtitles]
font = "Liberation Sans"
font_size = 48
color = "#FFFFFF"
border_color = "#000000"
border_size = 3.0
position = 95

[playback]
hwdec = "auto-safe"
audio_language = "eng"
sub_language = "eng"
volume = 100

[ui]
fullscreen = false
width = 1920
height = 1080
```

## Playback Controls

| Key | Action |
|-----|--------|
| Space | Play/Pause |
| Left/Right | Seek ±10s |
| Up/Down | Seek ±60s |
| +/- | Volume up/down |
| M | Mute |
| S | Cycle subtitles |
| A | Cycle audio tracks |
| F | Toggle fullscreen |
| Esc | Stop / Go back |

## License

MIT
