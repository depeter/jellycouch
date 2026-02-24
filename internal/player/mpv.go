package player

import (
	"fmt"
	"log"
	"sync"

	"github.com/gen2brain/go-mpv"

	"github.com/depeter/jellycouch/internal/config"
)

// Player wraps libmpv for video playback.
type Player struct {
	m        *mpv.Mpv
	mu       sync.Mutex
	playing  bool
	paused   bool
	duration float64
	position float64
	itemID   string

	OnPlaybackEnd func()
}

// New creates and initializes a new mpv player instance.
func New(cfg *config.Config) (*Player, error) {
	m := mpv.New()

	// Core options — mpv owns the render pipeline
	must(m.SetOptionString("hwdec", cfg.Playback.HWAccel))
	must(m.SetOptionString("vo", "gpu"))
	must(m.SetOptionString("input-vo-keyboard", "no"))
	must(m.SetOptionString("input-default-bindings", "no"))
	must(m.SetOptionString("osc", "no"))
	must(m.SetOptionString("osd-level", "1"))
	must(m.SetOptionString("osd-font", "Liberation Sans"))
	must(m.SetOptionString("osd-font-size", "32"))
	must(m.SetOptionString("osd-color", "#CCFFFFFF"))
	must(m.SetOptionString("osd-border-color", "#CC000000"))
	must(m.SetOptionString("osd-border-size", "2"))
	must(m.SetOptionString("osd-shadow-offset", "1"))
	must(m.SetOptionString("osd-shadow-color", "#80000000"))
	must(m.SetOptionString("osd-bar", "no"))
	must(m.SetOptionString("osd-align-y", "bottom"))
	must(m.SetOptionString("osd-margin-y", "40"))
	must(m.SetOptionString("keep-open", "yes"))
	must(m.SetOptionString("idle", "yes"))

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

	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("mpv init: %w", err)
	}

	p := &Player{m: m}

	// Observe properties for position/duration tracking
	m.ObserveProperty(0, "time-pos", mpv.FormatDouble)
	m.ObserveProperty(0, "duration", mpv.FormatDouble)
	m.ObserveProperty(0, "pause", mpv.FormatFlag)

	go p.eventLoop()

	return p, nil
}

func must(err error) {
	if err != nil {
		log.Printf("mpv option warning: %v", err)
	}
}

// SetWindowID sets the native window handle for embedded playback.
func (p *Player) SetWindowID(wid int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.SetOptionString("wid", fmt.Sprintf("%d", wid))
}

// LoadFile starts playback of a URL.
func (p *Player) LoadFile(url string, itemID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.itemID = itemID
	p.playing = true
	p.paused = false
	return p.m.Command([]string{"loadfile", url})
}

// Seek seeks relative to current position.
func (p *Player) Seek(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"seek", fmt.Sprintf("%.1f", seconds), "relative"})
}

// SeekAbsolute seeks to an absolute position.
func (p *Player) SeekAbsolute(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"seek", fmt.Sprintf("%.1f", seconds), "absolute"})
}

// TogglePause toggles pause state.
func (p *Player) TogglePause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"cycle", "pause"})
}

// SetVolume sets the volume (0-150).
func (p *Player) SetVolume(vol int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.SetPropertyString("volume", fmt.Sprintf("%d", vol))
}

// CycleSubtitles cycles through subtitle tracks.
func (p *Player) CycleSubtitles() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"cycle", "sub"})
}

// CycleAudio cycles through audio tracks.
func (p *Player) CycleAudio() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"cycle", "audio"})
}

// ToggleMute toggles audio mute.
func (p *Player) ToggleMute() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"cycle", "mute"})
}

// Stop stops playback.
func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playing = false
	return p.m.Command([]string{"stop"})
}

// Destroy cleans up the mpv instance.
func (p *Player) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m.TerminateDestroy()
}

// Volume returns the current volume level (0-150).
func (p *Player) Volume() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	vol := p.m.GetPropertyString("volume")
	var v int
	fmt.Sscanf(vol, "%d", &v)
	return v, nil
}

// Muted returns whether audio is muted.
func (p *Player) Muted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	val := p.m.GetPropertyString("mute")
	return val == "yes"
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

func (p *Player) eventLoop() {
	for {
		ev := p.m.WaitEvent(1.0)
		if ev == nil {
			continue
		}

		switch ev.EventID {
		case mpv.EventPropertyChange:
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
			p.mu.Lock()
			p.playing = false
			p.mu.Unlock()
			if p.OnPlaybackEnd != nil {
				p.OnPlaybackEnd()
			}

		case mpv.EventShutdown:
			return
		}
	}
}
