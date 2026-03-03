package player

import (
	"fmt"
	"time"
)

// renderPausedInfo renders the minimal bottom progress bar + time/duration
// and top-right clock as persistent OSD overlays while paused.
func (o *PlaybackOverlay) renderPausedInfo() {
	o.lastRender = time.Now()

	pos := o.player.Position()
	dur := o.player.Duration()
	clock := time.Now().Format("15:04")

	// Two ASS events in one overlay:
	// 1. Clock at top-right
	// 2. Progress bar + time/duration at bottom-center
	ass := fmt.Sprintf("{\\an9\\bord2\\fs%d%s}%s\n{\\an2\\bord2\\fs%d%s}%s\\N{\\fs%d%s}%s / %s",
		o.scale(22), assColorWhite, clock,
		o.scale(14), assColorGray, o.buildProgressBar(o.barWidth()),
		o.scale(17), assColorWhite, formatDuration(pos), formatDuration(dur))
	o.player.OsdOverlay(osdIDPausedBar, ass, o.screenW, o.screenH)
	o.pausedOsdShown = true
}

// hidePausedOsd removes the persistent paused overlay.
func (o *PlaybackOverlay) hidePausedOsd() {
	o.player.OsdOverlayRemove(osdIDPausedBar)
	o.pausedOsdShown = false
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
