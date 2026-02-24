package player

import (
	"fmt"
	"time"
)

// OSDState holds transient OSD display data.
type OSDState struct {
	ShowControls   bool
	ControlsTimer  time.Time
	ShowVolume     bool
	VolumeTimer    time.Time
}

// NewOSDState creates a new OSD state.
func NewOSDState() *OSDState {
	return &OSDState{}
}

// ShowControlsOverlay triggers the controls display.
func (o *OSDState) ShowControlsOverlay() {
	o.ShowControls = true
	o.ControlsTimer = time.Now().Add(3 * time.Second)
}

// ShowVolumeOverlay triggers the volume display.
func (o *OSDState) ShowVolumeOverlay() {
	o.ShowVolume = true
	o.VolumeTimer = time.Now().Add(2 * time.Second)
}

// Update checks timers and hides expired overlays.
func (o *OSDState) Update() {
	now := time.Now()
	if o.ShowControls && now.After(o.ControlsTimer) {
		o.ShowControls = false
	}
	if o.ShowVolume && now.After(o.VolumeTimer) {
		o.ShowVolume = false
	}
}

// FormatSeekBar generates an ASS-formatted seek bar string for mpv's osd-overlay.
func FormatSeekBar(position, duration float64, paused bool) string {
	if duration <= 0 {
		return ""
	}
	pct := position / duration
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	posStr := formatDuration(position)
	durStr := formatDuration(duration)
	status := "▶"
	if paused {
		status = "⏸"
	}

	// Bar using unicode block characters
	barWidth := 40
	filled := int(float64(barWidth) * pct)
	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else if i == filled {
			bar += "▓"
		} else {
			bar += "░"
		}
	}

	return fmt.Sprintf("%s  %s  %s  %s", status, posStr, bar, durStr)
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// SetOSDOverlay sends an OSD overlay command to mpv.
func (p *Player) SetOSDOverlay(id int, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if text == "" {
		return p.m.Command([]string{"osd-overlay", fmt.Sprintf("%d", id), "none", ""})
	}
	return p.m.Command([]string{"osd-overlay", fmt.Sprintf("%d", id), "ass-events", text})
}
