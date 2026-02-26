package player

import (
	"fmt"
	"strings"
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

// SetNextUp configures the next episode info for the "Up Next" banner.
func (o *PlaybackOverlay) SetNextUp(name string, index int) {
	o.nextUpName = name
	o.nextUpIndex = index
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
	b.WriteString("{\\an7\\bord0\\shad0}")

	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(13), assColorGray))
	b.WriteString("Up Next\\N")

	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s\\b1}", o.scale(15), assColorWhite))
	b.WriteString(fmt.Sprintf("Episode %d starting in %ds...{\\b0}\\N", o.nextUpIndex, remaining))

	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s\\b1}", o.scale(13), assColorBlue))
	b.WriteString("[ Start ]")

	o.player.ShowText(b.String(), 2000)
}
