package player

import (
	"fmt"
	"strings"
	"time"
)

// OverlayMode represents the current state of the playback overlay.
type OverlayMode int

const (
	OverlayHidden      OverlayMode = iota
	OverlayBar                     // Control bar visible at bottom
	OverlayTrackSelect             // Track selection panel (modal)
)

// TrackType distinguishes subtitle vs audio tracks.
type TrackType int

const (
	TrackSub   TrackType = iota
	TrackAudio
)

// ControlButton identifies a button on the control bar.
type ControlButton int

const (
	BtnSeekBack60  ControlButton = iota
	BtnSeekBack10
	BtnPlayPause
	BtnSeekFwd10
	BtnSeekFwd60
	BtnSubtitles
	BtnAudio
	btnCount // sentinel for wrapping
)

// Track holds metadata about an mpv track.
type Track struct {
	ID       int
	Type     TrackType
	Title    string
	Lang     string
	Codec    string
	Selected bool
	Default  bool
	Forced   bool
	External bool
}

// DisplayName returns a human-readable label for the track.
func (t Track) DisplayName() string {
	var parts []string

	name := t.Title
	if name == "" {
		name = langName(t.Lang)
	} else if lang := langName(t.Lang); lang != "" && lang != t.Title {
		name = t.Title + " - " + lang
	}
	if name == "" {
		name = fmt.Sprintf("Track %d", t.ID)
	}
	parts = append(parts, name)

	if t.Codec != "" {
		parts = append(parts, "["+t.Codec+"]")
	}

	var flags []string
	if t.Default {
		flags = append(flags, "default")
	}
	if t.Forced {
		flags = append(flags, "forced")
	}
	if t.External {
		flags = append(flags, "external")
	}
	if len(flags) > 0 {
		parts = append(parts, "("+strings.Join(flags, ", ")+")")
	}

	return strings.Join(parts, " ")
}

// Direction represents an input direction.
type Direction int

const (
	DirNone  Direction = iota
	DirLeft
	DirRight
	DirUp
	DirDown
)

// PlaybackOverlay manages the Kodi-style OSD rendered via mpv's show-text.
type PlaybackOverlay struct {
	player    *Player
	Mode      OverlayMode
	lastInput time.Time
	hideDelay time.Duration

	// Control bar state
	focusedBtn ControlButton

	// Track selection state
	trackType     TrackType
	tracks        []Track
	selectedIndex int
}

// NewPlaybackOverlay creates a new overlay for the given player.
func NewPlaybackOverlay(p *Player) *PlaybackOverlay {
	return &PlaybackOverlay{
		player:     p,
		Mode:       OverlayHidden,
		hideDelay:  4 * time.Second,
		focusedBtn: BtnPlayPause,
	}
}

// Show displays the control bar and resets the auto-hide timer.
func (o *PlaybackOverlay) Show() {
	o.Mode = OverlayBar
	o.lastInput = time.Now()
	o.renderBar()
}

// Hide clears the OSD and returns to hidden mode.
func (o *PlaybackOverlay) Hide() {
	o.Mode = OverlayHidden
	o.player.ShowText("", 1)
}

// Update checks the auto-hide timer. Call from the game loop.
func (o *PlaybackOverlay) Update() {
	if o.Mode == OverlayBar && time.Since(o.lastInput) > o.hideDelay {
		o.Hide()
	}
}

// HandleBarInput handles input when the control bar is visible.
// Returns true if the input was consumed.
func (o *PlaybackOverlay) HandleBarInput(dir Direction, enter, back bool) bool {
	o.lastInput = time.Now()

	if back || dir == DirUp {
		o.Hide()
		return true
	}

	if dir == DirLeft {
		if o.focusedBtn > 0 {
			o.focusedBtn--
		} else {
			o.focusedBtn = btnCount - 1
		}
		o.renderBar()
		return true
	}

	if dir == DirRight {
		o.focusedBtn++
		if o.focusedBtn >= btnCount {
			o.focusedBtn = 0
		}
		o.renderBar()
		return true
	}

	if enter {
		o.activateButton()
		return true
	}

	return false
}

// OpenTrackPanel fetches tracks and opens the selection panel.
func (o *PlaybackOverlay) OpenTrackPanel(tt TrackType) {
	o.trackType = tt
	o.tracks = o.player.GetTracks(tt)
	o.selectedIndex = 0

	// Find the currently selected track to pre-focus it
	for i, t := range o.tracks {
		if t.Selected {
			o.selectedIndex = i
			break
		}
	}

	o.Mode = OverlayTrackSelect
	o.lastInput = time.Now()
	o.renderTrackPanel()
}

// HandleTrackInput handles input when the track selection panel is open.
// Returns true if input was consumed.
func (o *PlaybackOverlay) HandleTrackInput(dir Direction, enter, back bool) bool {
	o.lastInput = time.Now()

	if back {
		// Close panel, return to control bar
		o.Mode = OverlayBar
		o.renderBar()
		return true
	}

	totalItems := len(o.tracks)
	if o.trackType == TrackSub {
		totalItems++ // "Off" option
	}

	if dir == DirUp {
		if o.selectedIndex > 0 {
			o.selectedIndex--
		}
		o.renderTrackPanel()
		return true
	}

	if dir == DirDown {
		if o.selectedIndex < totalItems-1 {
			o.selectedIndex++
		}
		o.renderTrackPanel()
		return true
	}

	if enter {
		o.selectTrack()
		return true
	}

	// Consume all other input while modal is open
	return true
}

// activateButton performs the action for the currently focused button.
func (o *PlaybackOverlay) activateButton() {
	switch o.focusedBtn {
	case BtnSeekBack60:
		o.player.Seek(-60)
		o.renderBar()
	case BtnSeekBack10:
		o.player.Seek(-10)
		o.renderBar()
	case BtnPlayPause:
		o.player.TogglePause()
		o.renderBar()
	case BtnSeekFwd10:
		o.player.Seek(10)
		o.renderBar()
	case BtnSeekFwd60:
		o.player.Seek(60)
		o.renderBar()
	case BtnSubtitles:
		o.OpenTrackPanel(TrackSub)
	case BtnAudio:
		o.OpenTrackPanel(TrackAudio)
	}
}

// selectTrack applies the selected track and closes the panel.
func (o *PlaybackOverlay) selectTrack() {
	if o.trackType == TrackSub {
		// Last item is "Off"
		if o.selectedIndex >= len(o.tracks) {
			o.player.SetSubTrack(0)
		} else {
			o.player.SetSubTrack(o.tracks[o.selectedIndex].ID)
		}
	} else {
		if o.selectedIndex < len(o.tracks) {
			o.player.SetAudioTrack(o.tracks[o.selectedIndex].ID)
		}
	}

	o.Mode = OverlayBar
	o.renderBar()
}

// ASS color constants (BGR format: &HBBGGRR&)
const (
	assColorBlue    = "\\1c&HDCA400&" // Jellyfin blue #00A4DC → BGR DC,A4,00
	assColorWhite   = "\\1c&HFFFFFF&"
	assColorGray    = "\\1c&H999999&"
	assColorDimGray = "\\1c&H666666&"
	assColorBarBg   = "\\1c&H333333&"
)

// renderBar renders the control bar ASS and sends it to mpv.
func (o *PlaybackOverlay) renderBar() {
	var b strings.Builder

	// ASS override prefix
	b.WriteString("${osd-ass-cc/0}")

	// Semi-transparent background strip at the bottom
	// Using \an2 for bottom-center alignment
	b.WriteString("{\\an2\\bord0\\shad0\\fsp0}")

	// Progress bar line
	b.WriteString("{\\fs15\\bord1" + assColorWhite + "}")
	b.WriteString(buildProgressBar(50) + "\\N")

	// Time and volume line
	b.WriteString("{\\fs17\\bord1" + assColorWhite + "}")
	b.WriteString("${time-pos} / ${duration}")
	b.WriteString("    ")
	b.WriteString("${?mute==yes:Muted}${!mute:Vol: ${volume}%}")
	b.WriteString("${?pause==yes:  \\N{\\fs14" + assColorGray + "$>Paused}")
	b.WriteString("\\N")

	// Button row
	b.WriteString("{\\fs18\\bord1}")
	buttons := []struct {
		btn   ControlButton
		label string
	}{
		{BtnSeekBack60, "\u25C0\u25C060"},
		{BtnSeekBack10, "\u25C010"},
		{BtnPlayPause, "\u25B6\u258E\u258E"},
		{BtnSeekFwd10, "10\u25B6"},
		{BtnSeekFwd60, "60\u25B6\u25B6"},
		{BtnSubtitles, "Subs"},
		{BtnAudio, "Audio"},
	}

	for i, btn := range buttons {
		if i > 0 {
			b.WriteString("{" + assColorDimGray + "}  \u2502  ")
		}
		if btn.btn == o.focusedBtn {
			b.WriteString("{" + assColorBlue + "\\b1}[ " + btn.label + " ]{\\b0}")
		} else {
			b.WriteString("{" + assColorGray + "}" + btn.label)
		}
	}

	b.WriteString("\\N")

	// Hint line
	b.WriteString("{\\fs13\\bord1" + assColorDimGray + "}")
	b.WriteString("\u2190 \u2192 Navigate   Enter Select   Esc Back")

	o.player.ShowText(b.String(), int(o.hideDelay.Milliseconds()+1000))
}

// renderTrackPanel renders the track selection panel ASS.
func (o *PlaybackOverlay) renderTrackPanel() {
	var b strings.Builder

	b.WriteString("${osd-ass-cc/0}")

	// Center-aligned panel
	b.WriteString("{\\an5\\bord0\\shad0}")

	// Title
	title := "Subtitle Tracks"
	if o.trackType == TrackAudio {
		title = "Audio Tracks"
	}
	b.WriteString("{\\fs20\\bord1" + assColorBlue + "}" + title + "\\N\\N")

	// Track list
	totalItems := len(o.tracks)
	if o.trackType == TrackSub {
		totalItems++ // "Off" option at the end
	}

	for i := 0; i < totalItems; i++ {
		var label string
		var isCurrentlyActive bool

		if o.trackType == TrackSub && i >= len(o.tracks) {
			// "Off" option for subtitles
			label = "Off"
			// Active if no track is selected
			isCurrentlyActive = true
			for _, t := range o.tracks {
				if t.Selected {
					isCurrentlyActive = false
					break
				}
			}
		} else {
			t := o.tracks[i]
			label = t.DisplayName()
			isCurrentlyActive = t.Selected
		}

		b.WriteString("{\\fs17\\bord1}")

		if i == o.selectedIndex {
			// Focused item: blue with arrow
			b.WriteString("{" + assColorBlue + "\\b1}")
			b.WriteString("\u25B8 " + label)
			b.WriteString("{\\b0}")
		} else if isCurrentlyActive {
			// Currently active: white with checkmark
			b.WriteString("{" + assColorWhite + "}")
			b.WriteString("\u2713 " + label)
		} else {
			// Normal item: gray
			b.WriteString("{" + assColorGray + "}")
			b.WriteString("   " + label)
		}

		b.WriteString("\\N")
	}

	// Hint line
	b.WriteString("\\N{\\fs13\\bord1" + assColorDimGray + "}")
	b.WriteString("\u2191\u2193 Navigate   Enter Select   Esc Back")

	o.player.ShowText(b.String(), 30000) // Long duration; we'll clear it manually
}

// buildProgressBar creates a Unicode block progress bar.
func buildProgressBar(width int) string {
	// We can't get live position in ASS static text, but mpv property expansion
	// doesn't work inside individual characters. Use a fixed-width bar and
	// let the time display show actual position.
	// Instead, use mpv's osd-bar for the actual progress and just show time.
	// Return empty — the time line already shows position.
	return ""
}

// formatDuration formats seconds into "H:MM:SS" or "MM:SS".
func formatDuration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// langName converts ISO 639-2/B language codes to human-readable names.
func langName(code string) string {
	if code == "" {
		return ""
	}
	langs := map[string]string{
		"eng": "English",
		"fre": "French",
		"fra": "French",
		"spa": "Spanish",
		"ger": "German",
		"deu": "German",
		"ita": "Italian",
		"por": "Portuguese",
		"rus": "Russian",
		"jpn": "Japanese",
		"kor": "Korean",
		"chi": "Chinese",
		"zho": "Chinese",
		"ara": "Arabic",
		"hin": "Hindi",
		"tur": "Turkish",
		"pol": "Polish",
		"dut": "Dutch",
		"nld": "Dutch",
		"swe": "Swedish",
		"nor": "Norwegian",
		"dan": "Danish",
		"fin": "Finnish",
		"hun": "Hungarian",
		"ces": "Czech",
		"cze": "Czech",
		"rum": "Romanian",
		"ron": "Romanian",
		"gre": "Greek",
		"ell": "Greek",
		"heb": "Hebrew",
		"tha": "Thai",
		"vie": "Vietnamese",
		"ind": "Indonesian",
		"may": "Malay",
		"msa": "Malay",
		"ukr": "Ukrainian",
		"bul": "Bulgarian",
		"hrv": "Croatian",
		"srp": "Serbian",
		"slv": "Slovenian",
		"slk": "Slovak",
		"slo": "Slovak",
		"cat": "Catalan",
		"fil": "Filipino",
		"tam": "Tamil",
		"tel": "Telugu",
		"ben": "Bengali",
		"und": "Unknown",
	}
	if name, ok := langs[code]; ok {
		return name
	}
	return code
}
