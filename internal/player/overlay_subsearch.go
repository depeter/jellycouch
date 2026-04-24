package player

import (
	"fmt"
	"strings"
	"time"
)

// OpenSubSearch switches the overlay into "searching online" mode. The caller
// is expected to follow up with SetSubSearchResults or SetSubSearchMessage
// once the search completes.
func (o *PlaybackOverlay) OpenSubSearch() {
	o.subSearchMu.Lock()
	o.subSearchLoading = true
	o.subSearchMsg = "Searching online subtitles…"
	o.subSearchResults = nil
	o.subSearchIndex = 0
	o.subSearchMu.Unlock()

	o.Mode = OverlaySubSearch
	o.lastInput = time.Now()
	o.renderSubSearch()
}

// SetSubSearchResults delivers async search results. Safe to call from any goroutine.
func (o *PlaybackOverlay) SetSubSearchResults(results []SubSearchEntry) {
	o.subSearchMu.Lock()
	o.subSearchLoading = false
	o.subSearchResults = results
	o.subSearchIndex = 0
	if len(results) == 0 {
		o.subSearchMsg = "No subtitles found"
	} else {
		o.subSearchMsg = ""
	}
	o.subSearchMu.Unlock()
	if o.Mode == OverlaySubSearch {
		o.renderSubSearch()
	}
}

// SetSubSearchMessage sets a status/error line on the search panel.
func (o *PlaybackOverlay) SetSubSearchMessage(msg string) {
	o.subSearchMu.Lock()
	o.subSearchMsg = msg
	o.subSearchMu.Unlock()
	if o.Mode == OverlaySubSearch {
		o.renderSubSearch()
	}
}

// SetSubSearchLoading toggles the loading indicator (e.g. while downloading).
func (o *PlaybackOverlay) SetSubSearchLoading(loading bool) {
	o.subSearchMu.Lock()
	o.subSearchLoading = loading
	o.subSearchMu.Unlock()
	if o.Mode == OverlaySubSearch {
		o.renderSubSearch()
	}
}

// CloseSubSearch returns the overlay to the control bar.
func (o *PlaybackOverlay) CloseSubSearch() {
	o.Mode = OverlayBar
	o.renderBar()
}

// HandleSubSearchInput processes keyboard input while the search panel is open.
// Always consumes input (returns true) since the panel is modal.
func (o *PlaybackOverlay) HandleSubSearchInput(dir Direction, enter, back bool) bool {
	o.lastInput = time.Now()

	if back {
		o.CloseSubSearch()
		return true
	}

	o.subSearchMu.Lock()
	n := len(o.subSearchResults)
	loading := o.subSearchLoading
	o.subSearchMu.Unlock()

	if loading {
		return true
	}

	switch dir {
	case DirUp:
		if n > 0 && o.subSearchIndex > 0 {
			o.subSearchIndex--
			o.renderSubSearch()
		}
	case DirDown:
		if n > 0 && o.subSearchIndex < n-1 {
			o.subSearchIndex++
			o.renderSubSearch()
		}
	}

	if enter && n > 0 && o.OnSubSearchDownload != nil {
		idx := o.subSearchIndex
		go o.OnSubSearchDownload(idx) // runs network work off the main thread
	}
	return true
}

// renderSubSearch draws the search results panel via mpv's OSD.
func (o *PlaybackOverlay) renderSubSearch() {
	o.subSearchMu.Lock()
	loading := o.subSearchLoading
	msg := o.subSearchMsg
	results := o.subSearchResults
	idx := o.subSearchIndex
	o.subSearchMu.Unlock()

	var b strings.Builder
	b.WriteString("{\\an5\\bord0\\shad0}")
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}Online Subtitles\\N\\N", o.scale(23), assColorBlue))

	if loading {
		b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}%s\\N", o.scale(20), assColorWhite, msg))
		o.player.OsdOverlay(osdIDMain, b.String(), o.screenW, o.screenH)
		return
	}

	if msg != "" && len(results) == 0 {
		b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}%s\\N", o.scale(20), assColorGray, msg))
		b.WriteString(fmt.Sprintf("\\N{\\fs%d\\bord1%s}Press Esc to close", o.scale(16), assColorDimGray))
		o.player.OsdOverlay(osdIDMain, b.String(), o.screenW, o.screenH)
		return
	}

	// Render up to a window of results around the focused index so the list
	// fits on screen on smaller resolutions.
	const windowSize = 12
	start := idx - windowSize/2
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end > len(results) {
		end = len(results)
		start = end - windowSize
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		r := results[i]
		line := fmt.Sprintf("[%s] %s  %s",
			strings.ToUpper(r.Language), r.Provider, truncateWide(r.ReleaseName, 60))
		if r.HearingImpaired {
			line += "  (SDH)"
		}
		if r.Downloads > 0 {
			line += fmt.Sprintf("  · %d downloads", r.Downloads)
		}

		b.WriteString(fmt.Sprintf("{\\fs%d\\bord1}", o.scale(18)))
		if i == idx {
			b.WriteString("{" + assColorBlue + "\\b1}▸ " + line + "{\\b0}")
		} else {
			b.WriteString("{" + assColorGray + "}   " + line)
		}
		b.WriteString("\\N")
	}

	// Footer
	b.WriteString("\\N")
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(16), assColorDimGray))
	if msg != "" {
		b.WriteString(msg + "\\N")
	}
	b.WriteString("↑/↓ Navigate · Enter Download · Esc Close")

	o.player.OsdOverlay(osdIDMain, b.String(), o.screenW, o.screenH)
}

// truncateWide shortens s to at most n runes, with an ellipsis. Kept local so
// rendering doesn't depend on unicode helpers.
func truncateWide(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
