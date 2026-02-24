package player

import (
	"fmt"
	"strings"
	"time"
)

// OSDState holds transient OSD display data.
type OSDState struct {
	ShowControls  bool
	ControlsTimer time.Time
	ShowVolume    bool
	VolumeTimer   time.Time
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

// FormatSeekBar generates a styled seek bar for mpv's show-text command.
// Uses a clean visual progress bar with Unicode drawing characters.
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
	remaining := duration - position
	if remaining < 0 {
		remaining = 0
	}
	remStr := "-" + formatDuration(remaining)

	// Status icon
	status := "\u25B6" // ▶
	if paused {
		status = "\u23F8" // ⏸
	}

	// Progress bar with smooth Unicode blocks
	// Use a mix of full/partial blocks for sub-character precision
	barWidth := 50
	filledExact := float64(barWidth) * pct
	filled := int(filledExact)
	fraction := filledExact - float64(filled)

	var bar strings.Builder
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar.WriteRune('\u2588') // █ full block
		} else if i == filled {
			// Partial block based on fraction
			switch {
			case fraction >= 0.875:
				bar.WriteRune('\u2588') // █
			case fraction >= 0.625:
				bar.WriteRune('\u2593') // ▓
			case fraction >= 0.375:
				bar.WriteRune('\u2592') // ▒
			case fraction >= 0.125:
				bar.WriteRune('\u2591') // ░
			default:
				bar.WriteRune('\u2015') // ― horizontal bar
			}
		} else {
			bar.WriteRune('\u2015') // ― horizontal bar
		}
	}

	// Clean layout:
	// ▶  0:42  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  -1:18:42  1:19:24
	return fmt.Sprintf("  %s  %s   %s   %s   %s",
		status, posStr, bar.String(), remStr, durStr)
}

// FormatVolumeOSD generates a volume indicator string.
func FormatVolumeOSD(volume int, muted bool) string {
	if muted {
		return "\U0001F507 Muted" // 🔇
	}
	// Volume bar
	barWidth := 20
	filled := volume * barWidth / 100
	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar.WriteRune('\u2588') // █
		} else {
			bar.WriteRune('\u2015') // ―
		}
	}
	icon := "\U0001F509" // 🔉
	if volume >= 70 {
		icon = "\U0001F50A" // 🔊
	} else if volume <= 0 {
		icon = "\U0001F507" // 🔇
	}
	return fmt.Sprintf("%s  %s  %d%%", icon, bar.String(), volume)
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

// ShowText displays text on the mpv OSD using the show-text command.
func (p *Player) ShowText(text string, durationMs int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"show-text", text, fmt.Sprintf("%d", durationMs)})
}

// ClearOSD clears the OSD text.
func (p *Player) ClearOSD() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Command([]string{"show-text", "", "1"})
}
