package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ErrorDisplay draws an error message with a "Copy" button.
// Store one per screen that shows errors, call Draw each frame and HandleClick in Update.
type ErrorDisplay struct {
	copyRect    ButtonRect
	copiedTimer int // frames remaining to show "Copied!" feedback
}

// Draw renders the error text and a Copy button. Returns the total height used.
// fontSize is typically FontSizeSmall or FontSizeBody.
func (ed *ErrorDisplay) Draw(dst *ebiten.Image, errText string, x, y, fontSize float64) float64 {
	if errText == "" {
		ed.copyRect = ButtonRect{}
		return 0
	}

	DrawText(dst, errText, x, y, fontSize, ColorError)

	// "Copy" button to the right of error text
	tw, _ := MeasureText(errText, fontSize)
	btnX := x + tw + 12
	btnY := y - 2
	btnW := 50.0
	btnH := fontSize + 6

	ed.copyRect = ButtonRect{X: btnX, Y: btnY, W: btnW, H: btnH}

	if ed.copiedTimer > 0 {
		ed.copiedTimer--
		DrawText(dst, "Copied!", btnX, y, FontSizeSmall, ColorSuccess)
	} else {
		vector.DrawFilledRect(dst, float32(btnX), float32(btnY), float32(btnW), float32(btnH), ColorSurface, false)
		vector.StrokeRect(dst, float32(btnX), float32(btnY), float32(btnW), float32(btnH), 1, ColorTextMuted, false)
		DrawTextCentered(dst, "Copy", btnX+btnW/2, btnY+btnH/2, FontSizeSmall, ColorTextSecondary)
	}

	return fontSize + 8
}

// HandleClick checks if the copy button was clicked. Call from Update with mouse coords.
// Returns true if the click was consumed.
func (ed *ErrorDisplay) HandleClick(mx, my int, errText string) bool {
	if errText == "" {
		return false
	}
	if PointInRect(mx, my, ed.copyRect.X, ed.copyRect.Y, ed.copyRect.W, ed.copyRect.H) {
		writeClipboard(errText)
		ed.copiedTimer = 120 // ~2 seconds at 60fps
		return true
	}
	return false
}
