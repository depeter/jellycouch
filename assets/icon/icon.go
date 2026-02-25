package icon

import (
	"image"
	"image/color"
	"math"
)

// Theme colors from the app
var (
	jellyfinBlue = color.RGBA{R: 0x00, G: 0xA4, B: 0xDC, A: 0xFF}
	purpleAccent = color.RGBA{R: 0xAA, G: 0x5C, B: 0xC3, A: 0xFF}
	darkBG       = color.RGBA{R: 0x10, G: 0x10, B: 0x14, A: 0xFF}
	couchDark    = color.RGBA{R: 0x00, G: 0x78, B: 0xA8, A: 0xFF}
	tentacleCol  = color.RGBA{R: 0x88, G: 0x44, B: 0xAA, A: 0xCC}
	glowCol      = color.RGBA{R: 0x00, G: 0xA4, B: 0xDC, A: 0x60}
)

// Generate returns 64x64 and 32x32 icon images for use with ebiten.SetWindowIcon.
func Generate() []image.Image {
	return []image.Image{
		generate(64),
		generate(32),
	}
}

func generate(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float64(size)

	// Fill background
	fillRect(img, 0, 0, size, size, darkBG)

	// Draw couch (lower portion)
	drawCouch(img, s)

	// Draw jellyfish on top of couch
	drawJellyfish(img, s)

	return img
}

func drawCouch(img *image.RGBA, s float64) {
	// Couch seat — wide rounded rectangle in lower third
	seatY := s * 0.58
	seatH := s * 0.18
	seatX := s * 0.10
	seatW := s * 0.80
	fillRoundedRect(img, seatX, seatY, seatW, seatH, s*0.06, jellyfinBlue)

	// Couch back — slightly narrower, behind the seat
	backY := s * 0.48
	backH := s * 0.14
	backX := s * 0.12
	backW := s * 0.76
	fillRoundedRect(img, backX, backY, backW, backH, s*0.05, couchDark)

	// Couch arms — two small rounded rects on sides
	armH := s * 0.22
	armW := s * 0.10
	armY := s * 0.48
	// Left arm
	fillRoundedRect(img, s*0.06, armY, armW, armH, s*0.04, couchDark)
	// Right arm
	fillRoundedRect(img, s*0.84, armY, armW, armH, s*0.04, couchDark)

	// Couch legs — small rectangles at bottom
	legW := s * 0.06
	legH := s * 0.10
	legY := s * 0.74
	fillRoundedRect(img, s*0.16, legY, legW, legH, s*0.02, couchDark)
	fillRoundedRect(img, s*0.78, legY, legW, legH, s*0.02, couchDark)

	// Cushion lines on seat
	lineY := int(seatY + seatH*0.3)
	lineEndY := int(seatY + seatH*0.7)
	for _, xf := range []float64{0.37, 0.50, 0.63} {
		x := int(s * xf)
		for y := lineY; y <= lineEndY; y++ {
			blendPixel(img, x, y, couchDark)
		}
	}
}

func drawJellyfish(img *image.RGBA, s float64) {
	// Jellyfish dome/bell — semicircle sitting on the couch
	domeCX := s * 0.50
	domeCY := s * 0.38
	domeR := s * 0.18

	// Glow behind dome
	fillCircle(img, domeCX, domeCY, domeR*1.25, glowCol)

	// Main dome
	fillDome(img, domeCX, domeCY, domeR, purpleAccent)

	// Highlight on dome
	highlightR := domeR * 0.35
	fillCircle(img, domeCX-domeR*0.25, domeCY-domeR*0.25, highlightR,
		color.RGBA{R: 0xCC, G: 0x88, B: 0xDD, A: 0x80})

	// Tentacles hanging down from the dome, draping over the front of the couch
	tentacleStartY := domeCY + domeR*0.1
	tentacleEndY := s * 0.72
	numTentacles := 5
	for i := 0; i < numTentacles; i++ {
		t := float64(i) / float64(numTentacles-1)
		tx := domeCX - domeR*0.7 + t*domeR*1.4
		// Wavy tentacles
		drawTentacle(img, tx, tentacleStartY, tentacleEndY, s*0.015, float64(i)*1.2, tentacleCol)
	}
}

func drawTentacle(img *image.RGBA, x, startY, endY, width, phase float64, c color.Color) {
	steps := int(endY - startY)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps)
		y := startY + float64(i)
		// Sine wave for wiggle
		waveX := x + math.Sin(t*math.Pi*2.5+phase)*width*2
		// Taper width toward end
		r := width * (1.0 - t*0.5)
		fillCircle(img, waveX, y, r, c)
	}
}

func fillRect(img *image.RGBA, x0, y0, w, h int, c color.Color) {
	bounds := img.Bounds()
	for y := y0; y < y0+h && y < bounds.Max.Y; y++ {
		for x := x0; x < x0+w && x < bounds.Max.X; x++ {
			if x >= 0 && y >= 0 {
				blendPixel(img, x, y, c)
			}
		}
	}
}

func fillRoundedRect(img *image.RGBA, xf, yf, wf, hf, rf float64, c color.Color) {
	x0 := int(xf)
	y0 := int(yf)
	x1 := int(xf + wf)
	y1 := int(yf + hf)
	r := rf
	bounds := img.Bounds()

	for y := y0; y <= y1 && y < bounds.Max.Y; y++ {
		for x := x0; x <= x1 && x < bounds.Max.X; x++ {
			if x < 0 || y < 0 {
				continue
			}
			// Check if inside rounded rect
			fx := float64(x)
			fy := float64(y)
			inside := true

			// Check corners
			if fx < xf+r && fy < yf+r {
				// Top-left corner
				dx := xf + r - fx
				dy := yf + r - fy
				if dx*dx+dy*dy > r*r {
					inside = false
				}
			} else if fx > xf+wf-r && fy < yf+r {
				// Top-right corner
				dx := fx - (xf + wf - r)
				dy := yf + r - fy
				if dx*dx+dy*dy > r*r {
					inside = false
				}
			} else if fx < xf+r && fy > yf+hf-r {
				// Bottom-left corner
				dx := xf + r - fx
				dy := fy - (yf + hf - r)
				if dx*dx+dy*dy > r*r {
					inside = false
				}
			} else if fx > xf+wf-r && fy > yf+hf-r {
				// Bottom-right corner
				dx := fx - (xf + wf - r)
				dy := fy - (yf + hf - r)
				if dx*dx+dy*dy > r*r {
					inside = false
				}
			}

			if inside {
				blendPixel(img, x, y, c)
			}
		}
	}
}

func fillCircle(img *image.RGBA, cx, cy, r float64, c color.Color) {
	bounds := img.Bounds()
	x0 := int(cx - r)
	y0 := int(cy - r)
	x1 := int(cx + r + 1)
	y1 := int(cy + r + 1)
	r2 := r * r

	for y := y0; y <= y1 && y < bounds.Max.Y; y++ {
		for x := x0; x <= x1 && x < bounds.Max.X; x++ {
			if x < 0 || y < 0 {
				continue
			}
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r2 {
				blendPixel(img, x, y, c)
			}
		}
	}
}

// fillDome draws the top half of a circle (a dome/bell shape).
func fillDome(img *image.RGBA, cx, cy, r float64, c color.Color) {
	bounds := img.Bounds()
	x0 := int(cx - r)
	y0 := int(cy - r)
	x1 := int(cx + r + 1)
	yCenterInt := int(cy + r*0.15) // Extend slightly below center for a bell shape
	r2 := r * r

	for y := y0; y <= yCenterInt && y < bounds.Max.Y; y++ {
		for x := x0; x <= x1 && x < bounds.Max.X; x++ {
			if x < 0 || y < 0 {
				continue
			}
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r2 {
				blendPixel(img, x, y, c)
			}
		}
	}
}

// blendPixel alpha-blends color c onto the existing pixel at (x, y).
func blendPixel(img *image.RGBA, x, y int, c color.Color) {
	r0, g0, b0, a0 := c.RGBA()
	if a0 == 0 {
		return
	}
	if a0 == 0xFFFF {
		img.Set(x, y, c)
		return
	}

	// Existing pixel
	existing := img.RGBAAt(x, y)
	er := uint32(existing.R) * 257
	eg := uint32(existing.G) * 257
	eb := uint32(existing.B) * 257

	// Alpha blend
	alpha := a0
	invAlpha := 0xFFFF - alpha
	nr := (r0*alpha + er*invAlpha) / 0xFFFF
	ng := (g0*alpha + eg*invAlpha) / 0xFFFF
	nb := (b0*alpha + eb*invAlpha) / 0xFFFF

	img.SetRGBA(x, y, color.RGBA{
		R: uint8(nr >> 8),
		G: uint8(ng >> 8),
		B: uint8(nb >> 8),
		A: 0xFF,
	})
}
