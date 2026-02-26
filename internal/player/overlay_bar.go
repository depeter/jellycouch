package player

import (
	"fmt"
	"strings"
	"time"
)

// ASS color constants (BGR format: &HBBGGRR&)
const (
	assColorBlue    = "\\1c&HDCA400&" // Jellyfin blue #00A4DC → BGR DC,A4,00
	assColorWhite   = "\\1c&HFFFFFF&"
	assColorGray    = "\\1c&H999999&"
	assColorDimGray = "\\1c&H666666&"
	assColorBarBg   = "\\1c&H333333&"
)

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

// activateButton performs the action for the currently focused button.
func (o *PlaybackOverlay) activateButton() {
	switch o.focusedBtn {
	case BtnSeekBack60:
		o.player.Seek(-SeekLarge)
		o.renderBar()
	case BtnSeekBack10:
		o.player.Seek(-SeekSmall)
		o.renderBar()
	case BtnPlayPause:
		o.player.TogglePause()
		o.renderBar()
	case BtnSeekFwd10:
		o.player.Seek(SeekSmall)
		o.renderBar()
	case BtnSeekFwd60:
		o.player.Seek(SeekLarge)
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

// btnCenterX estimates the pixel X coordinate of a button's center in the
// rendered button row.
func (o *PlaybackOverlay) btnCenterX(target ControlButton) int {
	btnLabels := map[ControlButton]int{
		BtnSeekBack60: 2, // ◀◀
		BtnSeekBack10: 1, // ◀
		BtnPlayPause:  1, // ⏸ or ▶
		BtnSeekFwd10:  1, // ▶
		BtnSeekFwd60:  2, // ▶▶
		BtnSubtitles:  1, // ☰
		BtnAudio:      1, // ♪
		BtnStop:       1, // ■
		BtnNext:       1, // ⏭
	}

	vis := o.visibleButtons()
	totalRunes := 0
	targetStart := 0
	targetLen := 0

	for i, btn := range vis {
		if i > 0 {
			totalRunes += 5 // "  │  "
		}
		labelLen := btnLabels[btn]
		if btn == target {
			targetStart = totalRunes
			targetLen = labelLen
		}
		totalRunes += labelLen
	}

	fontSize := float64(o.scale(12))
	charW := fontSize * 0.55
	barWidthPx := float64(totalRunes) * charW
	targetCenterRune := float64(targetStart) + float64(targetLen)/2.0
	barStartX := (float64(o.screenW) - barWidthPx) / 2.0

	return int(barStartX + targetCenterRune*charW)
}

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
		centerX := o.btnCenterX(BtnNext)
		imgX := centerX - epInfo.ImageW/2
		if imgX < 0 {
			imgX = 0
		} else if imgX+epInfo.ImageW > o.screenW {
			imgX = o.screenW - epInfo.ImageW
		}
		imgY := o.screenH - o.screenH*20/100 - epInfo.ImageH
		o.player.OverlayAdd(0, imgX, imgY, epInfo.ImagePath, epInfo.ImageW, epInfo.ImageH)
		o.imgOverlayShown = true
	} else if o.imgOverlayShown {
		o.player.OverlayRemove(0)
		o.imgOverlayShown = false
	}

	var b strings.Builder

	b.WriteString("${osd-ass-cc/0}")
	b.WriteString("{\\an2\\bord0\\shad0\\fsp0}")

	// Next episode tooltip line (above progress bar)
	if nextFocused {
		if epInfo != nil {
			tooltip := fmt.Sprintf("Up Next: S%dE%d \u00B7 %s",
				epInfo.SeasonNumber, epInfo.EpisodeNumber, epInfo.Title)
			b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(13), assColorWhite))
			b.WriteString(tooltip + "\\N")
		} else if noNext {
			b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(13), assColorDimGray))
			b.WriteString("No next episode\\N")
		}
	}

	// Progress bar line
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
	playPauseLabel := "\u23F8" // pause icon while playing
	if o.player.Paused() {
		playPauseLabel = "\u25B6" // play icon while paused
	}
	btnLabelMap := map[ControlButton]string{
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
		label := btnLabelMap[btn]
		if btn == BtnNext && noNext {
			b.WriteString("{" + assColorDimGray + "}" + label)
		} else if btn == o.focusedBtn {
			b.WriteString("{" + assColorBlue + "\\b1}" + label + "{\\b0}")
		} else {
			b.WriteString("{" + assColorGray + "}" + label)
		}
	}

	o.player.ShowText(b.String(), int(o.hideDelay.Milliseconds()+1000))
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
