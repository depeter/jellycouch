package player

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// NextEpisodeInfo holds pre-fetched metadata about the next episode.
type NextEpisodeInfo struct {
	Title         string
	SeasonNumber  int
	EpisodeNumber int
	ImagePath     string // path to raw BGRA temp file
	ImageW, ImageH int
	ItemID        string // Jellyfin item ID for direct playback
}

// OverlayMode represents the current state of the playback overlay.
type OverlayMode int

const (
	OverlayHidden      OverlayMode = iota
	OverlayBar                     // Control bar visible at bottom
	OverlayTrackSelect             // Track selection panel (modal)
	OverlayNextUp                  // "Up Next" countdown banner (top-left)
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
	BtnStop
	BtnNext
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

// FocusZone identifies which part of the overlay has input focus.
type FocusZone int

const (
	ZoneButtons  FocusZone = iota
	ZoneProgress
)

// seekAccel tracks acceleration state for rapid seek presses.
type seekAccel struct {
	lastDir   Direction
	lastPress time.Time
	stepIndex int
}

// seekSteps defines the escalating seek amounts in seconds.
var seekSteps = [...]float64{10, 30, 60, 300, 600}

// PlaybackOverlay manages the Kodi-style OSD rendered via mpv's show-text.
type PlaybackOverlay struct {
	player    *Player
	Mode      OverlayMode
	lastInput time.Time
	hideDelay time.Duration

	// Control bar state
	focusedBtn ControlButton
	focusZone  FocusZone
	accel      seekAccel
	lastRender time.Time

	// Screen dimensions for resolution-dependent scaling
	screenW, screenH int

	// Callbacks
	OnStop        func()
	OnNextEpisode func()
	OnStartNextUp func()

	// Next-up state
	nextUpName   string
	nextUpIndex  int
	nextUpActive bool

	// Next episode state
	nextEpMu       sync.Mutex
	nextEpInfo     *NextEpisodeInfo // pre-fetched info (nil = still loading)
	noNextEp       bool             // explicitly no next episode
	showNextBtn    bool             // whether to show BtnNext at all
	imgOverlayShown bool            // tracks whether overlay-add is active

	// Track selection state
	trackType     TrackType
	tracks        []Track
	selectedIndex int
}

// NewPlaybackOverlay creates a new overlay for the given player.
// screenW/screenH are used to scale font sizes proportionally to resolution.
func NewPlaybackOverlay(p *Player, screenW, screenH int) *PlaybackOverlay {
	if screenW <= 0 {
		screenW = 1920
	}
	if screenH <= 0 {
		screenH = 1080
	}
	return &PlaybackOverlay{
		player:     p,
		Mode:       OverlayHidden,
		hideDelay:  4 * time.Second,
		focusedBtn: BtnPlayPause,
		screenW:    screenW,
		screenH:    screenH,
	}
}

// scale returns a font size scaled proportionally from a 1080p baseline.
func (o *PlaybackOverlay) scale(base float64) int {
	s := base * float64(o.screenH) / 1080.0
	if s < 1 {
		s = 1
	}
	return int(s + 0.5)
}

// barWidth returns the progress bar character count scaled for resolution.
func (o *PlaybackOverlay) barWidth() int {
	w := 100 * o.screenW / 1920
	if w < 30 {
		w = 30
	}
	return w
}

// visibleButtons returns the set of buttons that should be shown, filtering
// out BtnNext when showNextBtn is false.
func (o *PlaybackOverlay) visibleButtons() []ControlButton {
	all := []ControlButton{
		BtnSeekBack60, BtnSeekBack10, BtnPlayPause, BtnStop,
		BtnSeekFwd10, BtnSeekFwd60, BtnSubtitles, BtnAudio,
	}
	if o.showNextBtn {
		all = append(all, BtnNext)
	}
	return all
}

// SetShowNextButton controls whether the Next button appears in the bar.
func (o *PlaybackOverlay) SetShowNextButton(show bool) {
	o.showNextBtn = show
}

// SetNextEpisode stores pre-fetched next episode info for the tooltip.
func (o *PlaybackOverlay) SetNextEpisode(info *NextEpisodeInfo) {
	o.nextEpMu.Lock()
	defer o.nextEpMu.Unlock()
	o.nextEpInfo = info
	o.noNextEp = false
}

// SetNoNextEpisode marks that there is no next episode available.
func (o *PlaybackOverlay) SetNoNextEpisode() {
	o.nextEpMu.Lock()
	defer o.nextEpMu.Unlock()
	o.nextEpInfo = nil
	o.noNextEp = true
}

// NextEpInfo returns the pre-fetched next episode info (nil if not yet loaded).
func (o *PlaybackOverlay) NextEpInfo() *NextEpisodeInfo {
	o.nextEpMu.Lock()
	defer o.nextEpMu.Unlock()
	return o.nextEpInfo
}

// NoNextEp returns true if there is explicitly no next episode.
func (o *PlaybackOverlay) NoNextEp() bool {
	o.nextEpMu.Lock()
	defer o.nextEpMu.Unlock()
	return o.noNextEp
}

// visibleIndex returns the index of focusedBtn in visibleButtons, or 0.
func (o *PlaybackOverlay) visibleIndex() int {
	for i, b := range o.visibleButtons() {
		if b == o.focusedBtn {
			return i
		}
	}
	return 0
}

// Show displays the control bar and resets the auto-hide timer.
func (o *PlaybackOverlay) Show() {
	o.Mode = OverlayBar
	o.lastInput = time.Now()
	o.focusZone = ZoneButtons
	o.accel = seekAccel{}
	o.renderBar()
}

// Hide clears the OSD and returns to hidden or next-up mode.
func (o *PlaybackOverlay) Hide() {
	if o.nextUpActive {
		o.Mode = OverlayNextUp
		o.renderNextUp()
		return
	}
	o.Mode = OverlayHidden
	o.player.ShowText("", 1)
	if o.imgOverlayShown {
		o.player.OverlayRemove(0)
		o.imgOverlayShown = false
	}
}

// SetNextUp configures the next episode info for the "Up Next" banner.
func (o *PlaybackOverlay) SetNextUp(name string, index int) {
	o.nextUpName = name
	o.nextUpIndex = index
}

// Update checks the auto-hide timer and next-up trigger. Call from the game loop.
func (o *PlaybackOverlay) Update() {
	if o.Mode == OverlayBar {
		if time.Since(o.lastInput) > o.hideDelay {
			o.Hide()
			return
		}
		// Periodically re-render to keep progress bar current
		if time.Since(o.lastRender) > time.Second {
			o.renderBar()
		}
	}

	// Check if we should activate the next-up banner
	if o.nextUpName != "" && !o.nextUpActive {
		pos := o.player.Position()
		dur := o.player.Duration()
		if dur > 0 && dur-pos <= 60 {
			o.nextUpActive = true
			if o.Mode == OverlayHidden {
				o.Mode = OverlayNextUp
				o.renderNextUp()
			}
		}
	}

	// Re-render next-up banner every second to update countdown
	if o.Mode == OverlayNextUp {
		if time.Since(o.lastRender) > time.Second {
			o.renderNextUp()
		}
	}
}

// HandleBarInput handles input when the control bar is visible.
// Returns true if the input was consumed.
func (o *PlaybackOverlay) HandleBarInput(dir Direction, enter, back bool) bool {
	o.lastInput = time.Now()

	if back {
		o.Hide()
		return true
	}

	switch o.focusZone {
	case ZoneButtons:
		if dir == DirUp {
			o.focusZone = ZoneProgress
			o.accel = seekAccel{}
			o.renderBar()
			return true
		}

		if dir == DirLeft {
			vis := o.visibleButtons()
			idx := o.visibleIndex()
			if idx > 0 {
				o.focusedBtn = vis[idx-1]
			} else {
				o.focusedBtn = vis[len(vis)-1]
			}
			o.renderBar()
			return true
		}

		if dir == DirRight {
			vis := o.visibleButtons()
			idx := o.visibleIndex()
			if idx < len(vis)-1 {
				o.focusedBtn = vis[idx+1]
			} else {
				o.focusedBtn = vis[0]
			}
			o.renderBar()
			return true
		}

		if enter {
			o.activateButton()
			return true
		}

	case ZoneProgress:
		if dir == DirUp {
			o.Hide()
			return true
		}

		if dir == DirDown {
			o.focusZone = ZoneButtons
			o.renderBar()
			return true
		}

		if dir == DirLeft {
			o.seekWithAcceleration(DirLeft)
			return true
		}

		if dir == DirRight {
			o.seekWithAcceleration(DirRight)
			return true
		}
	}

	return false
}

// seekWithAcceleration performs a seek with escalating step sizes on rapid presses.
func (o *PlaybackOverlay) seekWithAcceleration(dir Direction) {
	now := time.Now()

	if dir != o.accel.lastDir || o.accel.lastPress.IsZero() || now.Sub(o.accel.lastPress) > time.Second {
		// Direction changed, first press, or gap > 1s: reset
		o.accel.stepIndex = 0
	} else {
		// Same direction within 1s: escalate
		if o.accel.stepIndex < len(seekSteps)-1 {
			o.accel.stepIndex++
		}
	}

	o.accel.lastDir = dir
	o.accel.lastPress = now

	amount := seekSteps[o.accel.stepIndex]
	if dir == DirLeft {
		amount = -amount
	}
	o.player.Seek(amount)
	o.renderBar()
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
	case BtnStop:
		if o.OnStop != nil {
			o.OnStop()
		}
	case BtnNext:
		o.nextEpMu.Lock()
		hasNext := o.nextEpInfo != nil && !o.noNextEp
		o.nextEpMu.Unlock()
		if hasNext && o.OnNextEpisode != nil {
			o.OnNextEpisode()
		}
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
	o.lastRender = time.Now()

	// Snapshot next-episode state under lock
	o.nextEpMu.Lock()
	epInfo := o.nextEpInfo
	noNext := o.noNextEp
	o.nextEpMu.Unlock()

	// Manage image overlay for next-episode tooltip
	nextFocused := o.focusedBtn == BtnNext && o.showNextBtn && o.focusZone == ZoneButtons
	if nextFocused && epInfo != nil && epInfo.ImagePath != "" {
		// Position thumbnail centered above the bar area
		imgX := (o.screenW - epInfo.ImageW) / 2
		imgY := o.screenH - o.screenH*35/100 // ~35% from bottom
		o.player.OverlayAdd(0, imgX, imgY, epInfo.ImagePath, epInfo.ImageW, epInfo.ImageH)
		o.imgOverlayShown = true
	} else if o.imgOverlayShown {
		o.player.OverlayRemove(0)
		o.imgOverlayShown = false
	}

	var b strings.Builder

	// ASS override prefix
	b.WriteString("${osd-ass-cc/0}")

	// Semi-transparent background strip at the bottom
	// Using \an2 for bottom-center alignment
	b.WriteString("{\\an2\\bord0\\shad0\\fsp0}")

	// Next episode tooltip line (above progress bar)
	if nextFocused {
		if epInfo != nil {
			tooltip := fmt.Sprintf("Up Next: S%dE%d \u00B7 %s",
				epInfo.SeasonNumber, epInfo.EpisodeNumber, epInfo.Title)
			b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(11), assColorWhite))
			b.WriteString(tooltip + "\\N")
		} else if noNext {
			b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(11), assColorDimGray))
			b.WriteString("No next episode\\N")
		}
	}

	// Progress bar line (thin horizontal line characters)
	barColor := assColorGray
	if o.focusZone == ZoneProgress {
		barColor = assColorBlue
	}
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(9), barColor))
	b.WriteString(o.buildProgressBar(o.barWidth()) + "\\N")

	// Time and volume line
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(11), assColorWhite))
	b.WriteString("${time-pos} / ${duration}")
	b.WriteString("    ")
	b.WriteString("${?mute==yes:Muted}${!mute:Vol: ${volume}%}")
	b.WriteString(fmt.Sprintf("${?pause==yes:  \\N{\\fs%d%s$>Paused}", o.scale(10), assColorGray))
	b.WriteString("\\N")

	// Button row
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1}", o.scale(12)))
	playPauseLabel := "\u258E\u258E" // pause icon while playing
	if o.player.Paused() {
		playPauseLabel = "\u25B6" // play icon while paused
	}
	btnLabels := map[ControlButton]string{
		BtnSeekBack60: "\u25C0\u25C0",
		BtnSeekBack10: "\u25C0",
		BtnPlayPause:  playPauseLabel,
		BtnSeekFwd10:  "\u25B6",
		BtnSeekFwd60:  "\u25B6\u25B6",
		BtnSubtitles:  "\u2630",
		BtnAudio:      "\u266A",
		BtnStop:       "\u25A0",
		BtnNext:       "\u23ED",
	}

	for i, btn := range o.visibleButtons() {
		if i > 0 {
			b.WriteString("{" + assColorDimGray + "}  \u2502  ")
		}
		label := btnLabels[btn]
		if btn == BtnNext && noNext {
			// Dimmed next button when no next episode
			b.WriteString("{" + assColorDimGray + "}" + label)
		} else if btn == o.focusedBtn {
			b.WriteString("{" + assColorBlue + "\\b1}[ " + label + " ]{\\b0}")
		} else {
			b.WriteString("{" + assColorGray + "}" + label)
		}
	}

	o.player.ShowText(b.String(), int(o.hideDelay.Milliseconds()+1000))
}

// renderNextUp renders the "Up Next" countdown banner at top-left.
func (o *PlaybackOverlay) renderNextUp() {
	o.lastRender = time.Now()

	pos := o.player.Position()
	dur := o.player.Duration()
	remaining := int(dur - pos)
	if remaining < 0 {
		remaining = 0
	}

	var b strings.Builder
	b.WriteString("${osd-ass-cc/0}")

	// Top-left alignment
	b.WriteString("{\\an7\\bord0\\shad0}")

	// "Up Next" header
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(13), assColorGray))
	b.WriteString("Up Next\\N")

	// Episode name with countdown
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s\\b1}", o.scale(15), assColorWhite))
	b.WriteString(fmt.Sprintf("Episode %d starting in %ds...{\\b0}\\N", o.nextUpIndex, remaining))

	// Start button
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s\\b1}", o.scale(13), assColorBlue))
	b.WriteString("[ Start ]")

	o.player.ShowText(b.String(), 2000)
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
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(15), assColorBlue) + title + "\\N\\N")

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

		b.WriteString(fmt.Sprintf("{\\fs%d\\bord1}", o.scale(13)))

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

	o.player.ShowText(b.String(), 30000) // Long duration; we'll clear it manually
}

// buildProgressBar creates a Unicode block progress bar using current position/duration.
func (o *PlaybackOverlay) buildProgressBar(width int) string {
	pos := o.player.Position()
	dur := o.player.Duration()

	filled := 0
	if dur > 0 {
		frac := pos / dur
		if frac < 0 {
			frac = 0
		} else if frac > 1 {
			frac = 1
		}
		filled = int(frac*float64(width) + 0.5)
	}

	return strings.Repeat("\u2501", filled) + strings.Repeat("\u2500", width-filled)
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
