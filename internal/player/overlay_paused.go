package player

import (
	"fmt"
	"time"
)

// renderPausedInfo renders minimal time/duration + progress bar at the top
// and a top-right clock as persistent OSD overlays while paused.
func (o *PlaybackOverlay) renderPausedInfo() {
	o.lastRender = time.Now()

	pos := o.player.Position()
	dur := o.player.Duration()
	clock := time.Now().Format("15:04")

	// Single event using default osd-align (top-center, set in mpv init)
	ass := fmt.Sprintf("{\\bord2\\fs%d%s}%s / %s    %s\\N{\\fs%d%s}%s",
		o.scale(17), assColorWhite, formatDuration(pos), formatDuration(dur), clock,
		o.scale(14), assColorGray, o.buildProgressBar(o.barWidth()))
	o.player.OsdOverlay(osdIDMain, ass, o.screenW, o.screenH)
	o.pausedOsdShown = true
}

// hidePausedOsd removes the persistent paused overlay.
func (o *PlaybackOverlay) hidePausedOsd() {
	o.player.OsdOverlayRemove(osdIDMain)
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
