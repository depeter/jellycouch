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

// ASS color format: &HAABBGGRR (alpha, blue, green, red — yes, reversed from RGB)
const (
	assWhite      = "&H00FFFFFF"
	assWhiteDim   = "&H60FFFFFF"
	assBlack      = "&H00000000"
	assPrimary    = "&H00DCA400" // Jellyfin blue #00A4DC in BGR
	assPrimaryDim = "&H80DCA400"
	assBarBG      = "&H80000000" // semi-transparent black
	assBarTrack   = "&H60FFFFFF" // dim white track
	assShadow     = "&H80000000"
)

// FormatSeekBar generates a polished ASS-formatted seek bar for mpv's osd-overlay.
// Uses ASS vector drawing for a proper progress bar with rounded elements.
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

	// ASS PlayRes: 1920x1080 coordinate space
	// Layout: bottom-anchored bar with gradient backdrop
	var b strings.Builder

	// --- Background: full-width gradient panel at the bottom ---
	// Dark semi-transparent backdrop behind the controls
	// Using \an1 (bottom-left alignment) and \pos for precise placement
	b.WriteString(fmt.Sprintf(
		"{\\an5\\pos(960,1010)\\p1\\bord0\\shad0\\1c%s\\1a&H40&}m 0 0 l 1920 0 l 1920 140 l 0 140{\\p0}\n",
		assBlack,
	))

	// --- Progress bar track (background) ---
	// Thin rounded bar across the bottom area
	barX := 200    // left edge
	barW := 1520   // total width
	barY := 975    // vertical center of bar
	barH := 6      // bar height
	barR := 3      // corner radius

	// Track background (dim white)
	b.WriteString(fmt.Sprintf(
		"{\\an7\\pos(%d,%d)\\p1\\bord0\\shad0\\1c%s\\1a&H80&}%s{\\p0}\n",
		barX, barY-barH/2,
		assWhite,
		assRoundRect(0, 0, barW, barH, barR),
	))

	// --- Progress bar fill (Jellyfin blue) ---
	fillW := int(float64(barW) * pct)
	if fillW > 0 {
		if fillW < barR*2 {
			fillW = barR * 2
		}
		b.WriteString(fmt.Sprintf(
			"{\\an7\\pos(%d,%d)\\p1\\bord0\\shad0\\1c%s}%s{\\p0}\n",
			barX, barY-barH/2,
			assPrimary,
			assRoundRect(0, 0, fillW, barH, barR),
		))
	}

	// --- Scrubber dot at current position ---
	dotR := 10
	dotX := barX + int(float64(barW)*pct)
	b.WriteString(fmt.Sprintf(
		"{\\an5\\pos(%d,%d)\\p1\\bord0\\shad2\\3c%s\\1c%s}%s{\\p0}\n",
		dotX, barY,
		assShadow,
		assWhite,
		assCircle(0, 0, dotR),
	))

	// --- Play/Pause icon ---
	icon := "\u25B6" // ▶
	if paused {
		icon = "\u275A\u275A" // ❚❚
	}
	b.WriteString(fmt.Sprintf(
		"{\\an4\\pos(60,1000)\\bord0\\shad1\\3c%s\\fs42\\1c%s\\fnSegoe UI Symbol,Noto Sans Symbols2,sans-serif}%s{\\r}\n",
		assShadow, assWhite, icon,
	))

	// --- Current time ---
	b.WriteString(fmt.Sprintf(
		"{\\an4\\pos(120,1003)\\bord0\\shad1\\3c%s\\fs28\\1c%s\\fnSegoe UI,Liberation Sans,sans-serif\\b1}%s{\\r}\n",
		assShadow, assWhite, posStr,
	))

	// --- Duration (right-aligned) ---
	b.WriteString(fmt.Sprintf(
		"{\\an6\\pos(1860,1003)\\bord0\\shad1\\3c%s\\fs28\\1c%s\\fnSegoe UI,Liberation Sans,sans-serif}%s{\\r}\n",
		assShadow, assWhiteDim, durStr,
	))

	// --- Remaining time ---
	remaining := duration - position
	if remaining < 0 {
		remaining = 0
	}
	remStr := "-" + formatDuration(remaining)
	b.WriteString(fmt.Sprintf(
		"{\\an6\\pos(1740,1003)\\bord0\\shad1\\3c%s\\fs22\\1c%s\\fnSegoe UI,Liberation Sans,sans-serif}%s{\\r}\n",
		assShadow, assWhiteDim, remStr,
	))

	return b.String()
}

// FormatVolumeOSD generates an ASS-formatted volume indicator.
func FormatVolumeOSD(volume int, muted bool) string {
	var b strings.Builder

	// Volume icon + bar in top-right area
	label := fmt.Sprintf("Vol: %d%%", volume)
	if muted {
		label = "Muted"
	}

	// Background pill
	b.WriteString(fmt.Sprintf(
		"{\\an9\\pos(1880,60)\\p1\\bord0\\shad0\\1c%s\\1a&H40&}%s{\\p0}\n",
		assBlack,
		assRoundRect(0, 0, 200, 50, 12),
	))

	// Volume text
	b.WriteString(fmt.Sprintf(
		"{\\an9\\pos(1860,50)\\bord0\\shad1\\3c%s\\fs26\\1c%s\\fnSegoe UI,Liberation Sans,sans-serif\\b1}%s{\\r}\n",
		assShadow, assWhite, label,
	))

	return b.String()
}

// assRoundRect generates an ASS vector drawing for a rounded rectangle.
// Coordinates are relative to the \pos anchor.
func assRoundRect(x, y, w, h, r int) string {
	if r > h/2 {
		r = h / 2
	}
	if r > w/2 {
		r = w / 2
	}
	// ASS drawing: m = moveto, l = lineto, b = cubic bezier
	// Draw clockwise from top-left corner
	return fmt.Sprintf(
		"m %d %d l %d %d b %d %d %d %d %d %d l %d %d b %d %d %d %d %d %d l %d %d b %d %d %d %d %d %d l %d %d b %d %d %d %d %d %d",
		x+r, y,                  // start top-left after radius
		x+w-r, y,                // top edge
		x+w, y, x+w, y, x+w, y+r, // top-right corner
		x+w, y+h-r,             // right edge
		x+w, y+h, x+w, y+h, x+w-r, y+h, // bottom-right corner
		x+r, y+h,               // bottom edge
		x, y+h, x, y+h, x, y+h-r, // bottom-left corner
		x, y+r,                  // left edge
		x, y, x, y, x+r, y,     // top-left corner
	)
}

// assCircle generates an ASS vector drawing for a circle using cubic bezier curves.
func assCircle(cx, cy, r int) string {
	// Approximate circle with 4 cubic bezier segments
	// Control point distance for a circle: r * 0.5523
	k := r * 55 / 100
	return fmt.Sprintf(
		"m %d %d b %d %d %d %d %d %d b %d %d %d %d %d %d b %d %d %d %d %d %d b %d %d %d %d %d %d",
		cx, cy-r, // top
		cx+k, cy-r, cx+r, cy-k, cx+r, cy, // right
		cx+r, cy+k, cx+k, cy+r, cx, cy+r, // bottom
		cx-k, cy+r, cx-r, cy+k, cx-r, cy, // left
		cx-r, cy-k, cx-k, cy-r, cx, cy-r, // back to top
	)
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
