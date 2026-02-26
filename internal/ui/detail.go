package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ButtonRect stores the position and size of a drawn button.
type ButtonRect struct {
	X, Y, W, H float64
}

// DetailPanel shows item metadata with a backdrop.
type DetailPanel struct {
	Title          string
	Year           string
	Rating         string
	Runtime        string
	Overview       string
	RatingValue    float32
	OfficialRating string
	Genres         string
	Tagline        string
	Backdrop       *ebiten.Image
	ButtonIndex    int
	Buttons        []string
	ButtonRects    []ButtonRect // populated during Draw
}

func NewDetailPanel() *DetailPanel {
	return &DetailPanel{
		Buttons:     []string{"Play", "Resume", "Mark Watched"},
		ButtonIndex: 0,
	}
}

func (dp *DetailPanel) Update(dir Direction) bool {
	switch dir {
	case DirLeft:
		if dp.ButtonIndex > 0 {
			dp.ButtonIndex--
			return true
		}
	case DirRight:
		if dp.ButtonIndex < len(dp.Buttons)-1 {
			dp.ButtonIndex++
			return true
		}
	}
	return false
}

// HandleClick checks if (mx, my) hits any button and returns its index.
func (dp *DetailPanel) HandleClick(mx, my int) (buttonIndex int, ok bool) {
	for i, rect := range dp.ButtonRects {
		if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
			return i, true
		}
	}
	return -1, false
}

func (dp *DetailPanel) Draw(dst *ebiten.Image) {
	sw := float64(ScreenWidth)

	// Backdrop
	if dp.Backdrop != nil {
		op := &ebiten.DrawImageOptions{}
		bounds := dp.Backdrop.Bounds()
		scaleX := sw / float64(bounds.Dx())
		scaleY := float64(BackdropHeight) / float64(bounds.Dy())
		scale := scaleX
		if scaleY > scale {
			scale = scaleY
		}
		op.GeoM.Scale(scale, scale)
		op.Filter = ebiten.FilterLinear
		dst.DrawImage(dp.Backdrop, op)
	}

	// Gradient overlay at bottom of backdrop
	vector.DrawFilledRect(dst, 0, float32(BackdropHeight-100), float32(sw), 100,
		ColorOverlay, false)
	// Solid background below backdrop to cover image bleed and ensure readable text
	vector.DrawFilledRect(dst, 0, float32(BackdropHeight), float32(sw), float32(ScreenHeight-BackdropHeight),
		ColorBackground, false)

	// Metadata area below backdrop
	y := float64(BackdropHeight) + 20

	DrawText(dst, dp.Title, SectionPadding, y, FontSizeTitle, ColorText)
	y += FontSizeTitle + 8

	meta := ""
	if dp.Year != "" {
		meta += dp.Year
	}
	if dp.Runtime != "" {
		if meta != "" {
			meta += "  •  "
		}
		meta += dp.Runtime
	}
	if dp.OfficialRating != "" {
		if meta != "" {
			meta += "  •  "
		}
		meta += dp.OfficialRating
	}
	if dp.Genres != "" {
		if meta != "" {
			meta += "  •  "
		}
		meta += dp.Genres
	}
	if meta != "" {
		DrawText(dst, meta, SectionPadding, y, FontSizeBody, ColorTextSecondary)
	}
	// Draw rating with vector star after meta text
	if dp.RatingValue > 0 {
		ratingText := fmt.Sprintf("%.1f", dp.RatingValue)
		var metaEndX float64
		if meta != "" {
			tw, _ := MeasureText(meta, FontSizeBody)
			metaEndX = SectionPadding + tw + 20
		} else {
			metaEndX = SectionPadding
		}
		starR := float32(FontSizeBody * 0.4)
		starCX := float32(metaEndX) + starR
		starCY := float32(y) + float32(FontSizeBody)*0.45
		drawStarIcon(dst, starCX, starCY, starR, color.RGBA{R: 0xFF, G: 0xD7, B: 0x00, A: 0xFF})
		DrawText(dst, ratingText, float64(starCX+starR+4), y, FontSizeBody, ColorTextSecondary)
	}
	if meta != "" || dp.RatingValue > 0 {
		y += FontSizeBody + 12
	}

	// Tagline
	if dp.Tagline != "" {
		DrawText(dst, dp.Tagline, SectionPadding, y, FontSizeBody, ColorTextMuted)
		y += FontSizeBody + 8
	}

	// Overview (wrapped text)
	if dp.Overview != "" {
		maxW := sw - SectionPadding*2 - 400 // leave room on right
		h := DrawTextWrapped(dst, dp.Overview, SectionPadding, y, maxW, FontSizeBody, ColorTextSecondary)
		y += h + 16
	}

	// Action buttons — measure properly
	dp.ButtonRects = make([]ButtonRect, len(dp.Buttons))
	btnX := float64(SectionPadding)
	for i, label := range dp.Buttons {
		tw, _ := MeasureText(label, FontSizeBody)
		w := tw + 40
		h := float64(36)
		bx := float32(btnX)
		by := float32(y)

		dp.ButtonRects[i] = ButtonRect{X: btnX, Y: y, W: w, H: h}

		if i == dp.ButtonIndex {
			vector.DrawFilledRect(dst, bx, by, float32(w), float32(h), ColorPrimary, false)
			DrawTextCentered(dst, label, btnX+w/2, float64(by)+h/2, FontSizeBody, ColorText)
		} else {
			vector.DrawFilledRect(dst, bx, by, float32(w), float32(h), ColorSurface, false)
			DrawTextCentered(dst, label, btnX+w/2, float64(by)+h/2, FontSizeBody, ColorTextSecondary)
		}
		btnX += w + 12
	}
}

func FormatRuntime(ticks int64) string {
	minutes := ticks / 600_000_000
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}
