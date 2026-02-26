package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// drawCompassIcon draws a compass/discovery icon at (cx, cy) with given radius.
func drawCompassIcon(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	// Outer ring
	vector.StrokeCircle(dst, cx, cy, r, 1.5, clr, false)
	// Cardinal direction dots
	dotR := float32(1.5)
	vector.DrawFilledCircle(dst, cx, cy-r+2, dotR, clr, false) // N
	vector.DrawFilledCircle(dst, cx+r-2, cy, dotR, clr, false) // E
	vector.DrawFilledCircle(dst, cx, cy+r-2, dotR, clr, false) // S
	vector.DrawFilledCircle(dst, cx-r+2, cy, dotR, clr, false) // W
	// Diamond needle in center
	vector.StrokeLine(dst, cx, cy-3, cx+2, cy, 1.5, clr, false)
	vector.StrokeLine(dst, cx+2, cy, cx, cy+3, 1.5, clr, false)
	vector.StrokeLine(dst, cx, cy+3, cx-2, cy, 1.5, clr, false)
	vector.StrokeLine(dst, cx-2, cy, cx, cy-3, 1.5, clr, false)
}

// drawGearIcon draws a gear/settings icon at (cx, cy) with given radius.
func drawGearIcon(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	// Inner hub
	vector.DrawFilledCircle(dst, cx, cy, r*0.35, clr, false)
	// Outer teeth — small circles around the perimeter
	teeth := 8
	for i := 0; i < teeth; i++ {
		angle := float64(i) * 2 * math.Pi / float64(teeth)
		tx := cx + r*0.75*float32(math.Cos(angle))
		ty := cy + r*0.75*float32(math.Sin(angle))
		vector.DrawFilledCircle(dst, tx, ty, r*0.25, clr, false)
	}
	// Ring connecting teeth
	vector.StrokeCircle(dst, cx, cy, r*0.55, 1.5, clr, false)
}

// drawSearchIcon draws a magnifying glass icon at (cx, cy) with given radius.
func drawSearchIcon(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	// Lens circle (offset up-left so handle extends down-right)
	lensR := r * 0.6
	lensCX := cx - r*0.15
	lensCY := cy - r*0.15
	vector.StrokeCircle(dst, lensCX, lensCY, lensR, 1.8, clr, false)
	// Handle — line from bottom-right of lens at 45 degrees
	hx := lensCX + lensR*0.7
	hy := lensCY + lensR*0.7
	vector.StrokeLine(dst, hx, hy, hx+r*0.45, hy+r*0.45, 2, clr, false)
}

// drawListIcon draws a list/document icon at (cx, cy) with given radius.
func drawListIcon(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	// Three horizontal bars
	lineW := r * 1.2
	gap := r * 0.5
	for i := -1; i <= 1; i++ {
		ly := cy + float32(i)*gap
		// Bullet dot
		vector.DrawFilledCircle(dst, cx-lineW*0.6, ly, 1.5, clr, false)
		// Line
		vector.StrokeLine(dst, cx-lineW*0.3, ly, cx+lineW*0.7, ly, 1.8, clr, false)
	}
}

// drawTriangle draws a small filled triangle pointing up (if up=true) or down.
func drawTriangle(dst *ebiten.Image, cx, cy, size float32, up bool, clr color.Color) {
	var path vector.Path
	if up {
		path.MoveTo(cx, cy-size)
		path.LineTo(cx-size, cy+size*0.5)
		path.LineTo(cx+size, cy+size*0.5)
	} else {
		path.MoveTo(cx, cy+size)
		path.LineTo(cx-size, cy-size*0.5)
		path.LineTo(cx+size, cy-size*0.5)
	}
	path.Close()
	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	r, g, b, a := clr.RGBA()
	for i := range vs {
		vs[i].ColorR = float32(r) / 0xffff
		vs[i].ColorG = float32(g) / 0xffff
		vs[i].ColorB = float32(b) / 0xffff
		vs[i].ColorA = float32(a) / 0xffff
	}
	dst.DrawTriangles(vs, is, emptyImage, nil)
}

// drawStarIcon draws a filled 5-pointed star at (cx, cy) with given radius.
func drawStarIcon(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	var path vector.Path
	for i := 0; i < 5; i++ {
		// Outer point
		outerAngle := float64(i)*2*math.Pi/5 - math.Pi/2
		ox := cx + r*float32(math.Cos(outerAngle))
		oy := cy + r*float32(math.Sin(outerAngle))
		// Inner point (between outer points)
		innerAngle := outerAngle + math.Pi/5
		ir := r * 0.38
		ix := cx + ir*float32(math.Cos(innerAngle))
		iy := cy + ir*float32(math.Sin(innerAngle))
		if i == 0 {
			path.MoveTo(ox, oy)
		} else {
			path.LineTo(ox, oy)
		}
		path.LineTo(ix, iy)
	}
	path.Close()
	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	cr, cg, cb, ca := clr.RGBA()
	for i := range vs {
		vs[i].ColorR = float32(cr) / 0xffff
		vs[i].ColorG = float32(cg) / 0xffff
		vs[i].ColorB = float32(cb) / 0xffff
		vs[i].ColorA = float32(ca) / 0xffff
	}
	dst.DrawTriangles(vs, is, emptyImage, nil)
}

// emptyImage is a 1x1 white image for drawing arbitrary shapes.
var emptyImage = func() *ebiten.Image {
	img := ebiten.NewImage(1, 1)
	img.Fill(color.White)
	return img
}()

// drawCheckmark draws a small checkmark at (cx, cy) with given size.
func drawCheckmark(dst *ebiten.Image, cx, cy, size float32, clr color.Color) {
	// Short stroke down-right, then long stroke up-right
	vector.StrokeLine(dst, cx-size*0.5, cy, cx-size*0.1, cy+size*0.5, 2, clr, false)
	vector.StrokeLine(dst, cx-size*0.1, cy+size*0.5, cx+size*0.6, cy-size*0.4, 2, clr, false)
}

// drawXMark draws a small X at (cx, cy) with given size.
func drawXMark(dst *ebiten.Image, cx, cy, size float32, clr color.Color) {
	vector.StrokeLine(dst, cx-size, cy-size, cx+size, cy+size, 1.5, clr, false)
	vector.StrokeLine(dst, cx+size, cy-size, cx-size, cy+size, 1.5, clr, false)
}

// drawNavButton draws a styled nav bar button and returns its bounds.
func drawNavButton(dst *ebiten.Image, label string, x, y, w, h float32, focused bool, iconFn func(*ebiten.Image, float32, float32, float32, color.Color), accentColor color.Color) {
	if focused {
		vector.DrawFilledRect(dst, x, y, w, h, ColorPrimary, false)
		DrawTextCentered(dst, label, float64(x+w/2+8), float64(y+h/2), FontSizeBody, ColorBackground)
		if iconFn != nil {
			iconFn(dst, x+16, y+h/2, 7, ColorBackground)
		}
	} else {
		vector.DrawFilledRect(dst, x, y, w, h, ColorSurfaceHover, false)
		vector.StrokeRect(dst, x, y, w, h, 1, accentColor, false)
		DrawTextCentered(dst, label, float64(x+w/2+8), float64(y+h/2), FontSizeBody, ColorText)
		if iconFn != nil {
			iconFn(dst, x+16, y+h/2, 7, accentColor)
		}
	}
}
