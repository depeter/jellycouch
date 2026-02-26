package player

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/go-mpv"

	"github.com/depeter/jellycouch/internal/config"
)

// playerCmd is a function to execute on the mpv thread, with a channel for the result.
type playerCmd struct {
	fn     func(m *mpv.Mpv) error
	result chan error
}

// Player wraps libmpv for video playback.
// All mpv API calls are proxied to a single dedicated OS thread.
type Player struct {
	cmdCh chan playerCmd

	mu       sync.Mutex
	playing  bool
	paused   bool
	duration float64
	position float64
	itemID   string

	OnPlaybackEnd func()
}

// New creates and initializes a new mpv player instance.
// The mpv handle is created, configured, and used entirely on a single OS thread.
func New(cfg *config.Config) (*Player, error) {
	p := &Player{
		cmdCh: make(chan playerCmd, 8),
	}

	initErr := make(chan error, 1)
	go p.mpvThread(cfg, initErr)

	if err := <-initErr; err != nil {
		return nil, err
	}
	return p, nil
}

func must(err error) {
	if err != nil {
		log.Printf("mpv option warning: %v", err)
	}
}

// mpvCmd builds a quoted command string for mpv_command_string.
// This avoids go-mpv's Command() which has a missing NULL terminator
// in its nocgo char** array on Windows.
func mpvCmd(args ...string) string {
	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('"')
		for _, c := range arg {
			if c == '"' || c == '\\' {
				b.WriteByte('\\')
			}
			b.WriteRune(c)
		}
		b.WriteByte('"')
	}
	return b.String()
}

// mpvThread runs on a locked OS thread. All mpv API calls happen here.
func (p *Player) mpvThread(cfg *config.Config, initErr chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	m := mpv.New()

	// Core options — mpv owns the render pipeline
	must(m.SetOptionString("hwdec", cfg.Playback.HWAccel))
	must(m.SetOptionString("vo", "gpu"))
	must(m.SetOptionString("keep-open", "yes"))
	must(m.SetOptionString("idle", "yes"))

	// Disable mpv's own key handling — we forward keys from Ebitengine
	// so they work on all platforms (Windows embedding doesn't pass keys)
	must(m.SetOptionString("input-default-bindings", "no"))
	must(m.SetOptionString("input-vo-keyboard", "no"))

	// Enable OSD bar for seek/volume feedback
	must(m.SetOptionString("osd-level", "1"))
	must(m.SetOptionString("osd-duration", "2000"))
	must(m.SetOptionString("osd-bar", "yes"))

	// Subtitle defaults from config
	must(m.SetOptionString("sub-font", cfg.Subtitles.Font))
	must(m.SetOptionString("sub-font-size", fmt.Sprintf("%d", cfg.Subtitles.FontSize)))
	must(m.SetOptionString("sub-color", cfg.Subtitles.Color))
	must(m.SetOptionString("sub-border-color", cfg.Subtitles.BorderColor))
	must(m.SetOptionString("sub-border-size", fmt.Sprintf("%.1f", cfg.Subtitles.BorderSize)))
	must(m.SetOptionString("sub-shadow-offset", fmt.Sprintf("%.1f", cfg.Subtitles.ShadowOffset)))
	must(m.SetOptionString("sub-pos", fmt.Sprintf("%d", cfg.Subtitles.Position)))
	if cfg.Subtitles.ASSOverride != "" {
		must(m.SetOptionString("sub-ass-override", cfg.Subtitles.ASSOverride))
	}

	// Audio/sub language preferences
	if cfg.Playback.AudioLanguage != "" {
		must(m.SetOptionString("alang", cfg.Playback.AudioLanguage))
	}
	if cfg.Playback.SubLanguage != "" {
		must(m.SetOptionString("slang", cfg.Playback.SubLanguage))
	}

	// Volume
	must(m.SetOptionString("volume", fmt.Sprintf("%d", cfg.Playback.Volume)))

	// Enable yt-dlp for YouTube URLs (trailers, etc.)
	must(m.SetOptionString("ytdl", "yes"))

	if err := m.Initialize(); err != nil {
		initErr <- fmt.Errorf("mpv init: %w", err)
		return
	}

	// Observe properties for position/duration tracking
	m.ObserveProperty(0, "time-pos", mpv.FormatDouble)
	m.ObserveProperty(0, "duration", mpv.FormatDouble)
	m.ObserveProperty(0, "pause", mpv.FormatFlag)

	initErr <- nil

	// Combined event + command loop.
	// Use WaitEvent(0) (non-blocking poll) to avoid purego float64
	// argument issues on Windows, then sleep between iterations.
	for {
		// Process any pending command immediately
		select {
		case cmd := <-p.cmdCh:
			cmd.result <- cmd.fn(m)
			continue
		default:
		}

		// Poll for mpv events (non-blocking: timeout=0)
		ev := m.WaitEvent(0)
		if ev == nil || ev.EventID == mpv.EventNone {
			// No events and no commands — wait for a command or poll again shortly
			select {
			case cmd := <-p.cmdCh:
				cmd.result <- cmd.fn(m)
			case <-time.After(16 * time.Millisecond):
			}
			continue
		}

		switch ev.EventID {
		case mpv.EventPropertyChange:
			if ev.Data == nil {
				continue
			}
			prop := ev.Property()
			p.mu.Lock()
			switch prop.Name {
			case "time-pos":
				if v, ok := prop.Data.(float64); ok {
					p.position = v
				}
			case "duration":
				if v, ok := prop.Data.(float64); ok {
					p.duration = v
				}
			case "pause":
				if v, ok := prop.Data.(int); ok {
					p.paused = v == 1
				}
			}
			p.mu.Unlock()

		case mpv.EventEnd:
			if ev.Data == nil {
				p.mu.Lock()
				wasPlaying := p.playing
				p.playing = false
				p.mu.Unlock()
				if wasPlaying && p.OnPlaybackEnd != nil {
					p.OnPlaybackEnd()
				}
				continue
			}
			ef := ev.EndFile()
			p.mu.Lock()
			wasPlaying := p.playing
			p.playing = false
			p.mu.Unlock()
			log.Printf("mpv end-file: reason=%s wasPlaying=%v", ef.Reason, wasPlaying)
			if wasPlaying && p.OnPlaybackEnd != nil {
				p.OnPlaybackEnd()
			}

		case mpv.EventShutdown:
			return
		}
	}
}

// do sends a command to the mpv thread and waits for the result.
func (p *Player) do(fn func(m *mpv.Mpv) error) error {
	ch := make(chan error, 1)
	p.cmdCh <- playerCmd{fn: fn, result: ch}
	return <-ch
}

// SetWindowID sets the native window handle for embedded playback.
func (p *Player) SetWindowID(wid int64) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.SetOptionString("wid", fmt.Sprintf("%d", wid))
	})
}

// LoadFile starts playback of a URL. If startSeconds > 0 the file opens at that position.
func (p *Player) LoadFile(url string, itemID string, startSeconds float64) error {
	p.mu.Lock()
	p.itemID = itemID
	p.playing = true
	p.paused = false
	p.mu.Unlock()
	return p.do(func(m *mpv.Mpv) error {
		if startSeconds > 0 {
			return m.CommandString(mpvCmd("loadfile", url, "replace", fmt.Sprintf("start=%.1f", startSeconds)))
		}
		return m.CommandString(mpvCmd("loadfile", url))
	})
}

// Seek seeks relative to current position.
func (p *Player) Seek(seconds float64) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("seek", fmt.Sprintf("%.1f", seconds), "relative"))
	})
}

// SeekAbsolute seeks to an absolute position.
func (p *Player) SeekAbsolute(seconds float64) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("seek", fmt.Sprintf("%.1f", seconds), "absolute"))
	})
}

// TogglePause toggles pause state.
func (p *Player) TogglePause() error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("cycle", "pause"))
	})
}

// SetVolume sets the volume (0-150).
func (p *Player) SetVolume(vol int) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.SetPropertyString("volume", fmt.Sprintf("%d", vol))
	})
}

// AdjustVolume changes volume by a relative amount.
func (p *Player) AdjustVolume(delta int) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("add", "volume", fmt.Sprintf("%d", delta)))
	})
}

// ShowProgress flashes the OSD progress bar.
func (p *Player) ShowProgress() {
	p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("show-progress"))
	})
}

// OverlayAdd displays a raw BGRA image overlay on the mpv window.
// id is an overlay slot (0-63), x/y are pixel coordinates, filePath points to
// a file containing raw BGRA pixel data, and w/h are the image dimensions.
func (p *Player) OverlayAdd(id, x, y int, filePath string, w, h int) {
	stride := w * 4
	p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("overlay-add",
			fmt.Sprintf("%d", id),
			fmt.Sprintf("%d", x),
			fmt.Sprintf("%d", y),
			filePath,
			"0",
			"bgra",
			fmt.Sprintf("%d", w),
			fmt.Sprintf("%d", h),
			fmt.Sprintf("%d", stride),
		))
	})
}

// OverlayRemove removes a previously added image overlay by slot id.
func (p *Player) OverlayRemove(id int) {
	p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("overlay-remove", fmt.Sprintf("%d", id)))
	})
}

// ShowText displays a text message on mpv's OSD for the given duration (ms).
func (p *Player) ShowText(text string, durationMS int) {
	p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("show-text", text, fmt.Sprintf("%d", durationMS)))
	})
}

// ShowOSD displays a status overlay with playback info and key hints.
// Uses mpv's property expansion to show live values.
func (p *Player) ShowOSD() {
	// ${time-pos} and ${duration} are expanded by mpv at display time.
	// ${?pause==yes:⏸ Paused} is conditional: shown only when paused.
	osd := "${osd-ass-cc/0}" +
		"{\\an2\\fs28\\bord2}" +
		"${time-pos} / ${duration}   Vol: ${volume}%\\N" +
		"{\\fs22\\alpha&H40&}" +
		"Left/Right Seek   Space Pause   S Subs   A Audio   Esc Back"
	p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("show-text", osd, "4000"))
	})
}

// CycleSubtitles cycles through subtitle tracks.
func (p *Player) CycleSubtitles() error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("cycle", "sub"))
	})
}

// CycleAudio cycles through audio tracks.
func (p *Player) CycleAudio() error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("cycle", "audio"))
	})
}

// GetTracks returns all tracks of the given type from mpv's track list.
func (p *Player) GetTracks(trackType TrackType) []Track {
	var tracks []Track
	p.do(func(m *mpv.Mpv) error {
		countStr := m.GetPropertyString("track-list/count")
		count := 0
		fmt.Sscanf(countStr, "%d", &count)

		for i := 0; i < count; i++ {
			prefix := fmt.Sprintf("track-list/%d/", i)
			typ := m.GetPropertyString(prefix + "type")

			wantType := "sub"
			if trackType == TrackAudio {
				wantType = "audio"
			}
			if typ != wantType {
				continue
			}

			id := 0
			fmt.Sscanf(m.GetPropertyString(prefix+"id"), "%d", &id)

			t := Track{
				ID:       id,
				Type:     trackType,
				Title:    m.GetPropertyString(prefix + "title"),
				Lang:     m.GetPropertyString(prefix + "lang"),
				Codec:    m.GetPropertyString(prefix + "codec"),
				Selected: m.GetPropertyString(prefix+"selected") == "yes",
				Default:  m.GetPropertyString(prefix+"default") == "yes",
				Forced:   m.GetPropertyString(prefix+"forced") == "yes",
				External: m.GetPropertyString(prefix+"external") == "yes",
			}
			tracks = append(tracks, t)
		}
		return nil
	})
	return tracks
}

// SetSubTrack sets the subtitle track. id=0 disables subtitles.
func (p *Player) SetSubTrack(id int) error {
	return p.do(func(m *mpv.Mpv) error {
		if id == 0 {
			return m.SetPropertyString("sid", "no")
		}
		return m.SetPropertyString("sid", fmt.Sprintf("%d", id))
	})
}

// SetAudioTrack sets the audio track by ID.
func (p *Player) SetAudioTrack(id int) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.SetPropertyString("aid", fmt.Sprintf("%d", id))
	})
}

// ToggleMute toggles audio mute.
func (p *Player) ToggleMute() error {
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("cycle", "mute"))
	})
}

// Stop stops playback.
func (p *Player) Stop() error {
	p.mu.Lock()
	p.playing = false
	p.mu.Unlock()
	return p.do(func(m *mpv.Mpv) error {
		return m.CommandString(mpvCmd("stop"))
	})
}

// Destroy cleans up the mpv instance.
func (p *Player) Destroy() {
	p.do(func(m *mpv.Mpv) error {
		m.TerminateDestroy()
		return nil
	})
}

// Playing returns whether media is currently loaded.
func (p *Player) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

// Paused returns the current pause state.
func (p *Player) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// Position returns the current playback position in seconds.
func (p *Player) Position() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.position
}

// Duration returns the total duration in seconds.
func (p *Player) Duration() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.duration
}

// ItemID returns the currently playing item ID.
func (p *Player) ItemID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.itemID
}
