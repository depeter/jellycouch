package player

import (
	"sync"
	"time"
)

// OverlayMode represents the current state of the playback overlay.
type OverlayMode int

const (
	OverlayHidden      OverlayMode = iota
	OverlayBar                     // Control bar visible at bottom
	OverlayTrackSelect             // Track selection panel (modal)
	OverlayNextUp                  // "Up Next" countdown banner (top-left)
)

// OSD overlay slot IDs for persistent overlays (via osd-overlay command).
const (
	osdIDClock     = 1
	osdIDPausedBar = 2
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

// Playback control constants.
const (
	SeekSmall  = 10  // seconds for small seek
	SeekLarge  = 60  // seconds for large seek
	VolumeStep = 5   // percent per volume adjustment
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

	// Paused persistent OSD state
	pausedOsdShown bool

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
	// Remove paused bar (full bar replaces it), but keep clock
	o.player.OsdOverlayRemove(osdIDPausedBar)
	if o.player.Paused() {
		o.renderClock()
	} else {
		o.hidePausedOsd()
	}
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
	// If paused, immediately show the minimal paused info
	if o.player.Paused() {
		o.renderPausedInfo()
	}
}

// Update checks the auto-hide timer and next-up trigger. Call from the game loop.
func (o *PlaybackOverlay) Update() {
	paused := o.player.Paused()

	if o.Mode == OverlayBar {
		if time.Since(o.lastInput) > o.hideDelay {
			o.Hide()
			return
		}
		// Periodically re-render to keep progress bar current
		if time.Since(o.lastRender) > time.Second {
			o.renderBar()
			if paused {
				o.renderClock()
			}
		}
	}

	// Paused + hidden: show persistent minimal info, updated every second
	if paused && o.Mode == OverlayHidden {
		if time.Since(o.lastRender) > time.Second {
			o.renderPausedInfo()
		}
	}

	// Not paused: ensure persistent overlays are removed
	if !paused && o.pausedOsdShown {
		o.hidePausedOsd()
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

// Cleanup removes all persistent overlays. Call before discarding the overlay.
func (o *PlaybackOverlay) Cleanup() {
	o.hidePausedOsd()
}
