package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DrawButton renders a filled rectangle with centered text. Focused buttons
// use the primary accent; others use the muted surface. Text is drawn at
// FontSizeBody by default — pass a non-zero fontSize to override.
func DrawButton(dst *ebiten.Image, rect ButtonRect, label string, focused bool, fontSize float64) {
	if fontSize == 0 {
		fontSize = FontSizeBody
	}
	bg := ColorSurface
	fg := ColorTextSecondary
	if focused {
		bg = ColorPrimary
		fg = ColorText
	}
	vector.DrawFilledRect(dst, float32(rect.X), float32(rect.Y), float32(rect.W), float32(rect.H), bg, false)
	DrawTextCentered(dst, label, rect.X+rect.W/2, rect.Y+rect.H/2, fontSize, fg)
}

// LayoutButtonRow computes ButtonRects for a horizontal row of labels. Each
// button is sized to its text width plus horizontal padding; buttons are
// spaced by gap. Returns the slice of rects and the total right edge.
func LayoutButtonRow(labels []string, startX, y, height, padX, gap, fontSize float64) (rects []ButtonRect, endX float64) {
	if fontSize == 0 {
		fontSize = FontSizeBody
	}
	rects = make([]ButtonRect, len(labels))
	x := startX
	for i, label := range labels {
		tw, _ := MeasureText(label, fontSize)
		w := tw + padX*2
		rects[i] = ButtonRect{X: x, Y: y, W: w, H: height}
		x += w + gap
	}
	return rects, x
}

// HitButton returns the index of the rect containing (mx, my), or -1.
func HitButton(rects []ButtonRect, mx, my int) int {
	for i, r := range rects {
		if PointInRect(mx, my, r.X, r.Y, r.W, r.H) {
			return i
		}
	}
	return -1
}
