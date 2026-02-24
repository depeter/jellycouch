package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// DetailPanel shows item metadata with a backdrop.
type DetailPanel struct {
	Title       string
	Year        string
	Rating      string
	Runtime     string
	Overview    string
	Backdrop    *ebiten.Image
	ButtonIndex int
	Buttons     []string
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
		dst.DrawImage(dp.Backdrop, op)
	}

	// Gradient overlay on backdrop
	vector.DrawFilledRect(dst, 0, float32(BackdropHeight-100), float32(sw), 100,
		ColorOverlay, false)

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
	if dp.Rating != "" {
		if meta != "" {
			meta += "  •  "
		}
		meta += dp.Rating
	}
	if meta != "" {
		DrawText(dst, meta, SectionPadding, y, FontSizeBody, ColorTextSecondary)
		y += FontSizeBody + 12
	}

	// Overview (wrapped text)
	if dp.Overview != "" {
		maxW := sw - SectionPadding*2 - 400 // leave room on right
		h := DrawTextWrapped(dst, dp.Overview, SectionPadding, y, maxW, FontSizeBody, ColorTextSecondary)
		y += h + 16
	}

	// Action buttons
	btnX := float64(SectionPadding)
	for i, label := range dp.Buttons {
		w := float64(len(label)*10 + 40)
		h := float64(36)
		bx := float32(btnX)
		by := float32(y)

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
