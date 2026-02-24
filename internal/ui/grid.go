package ui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// GridItem represents a single item in a poster grid.
type GridItem struct {
	ID       string
	Title    string
	Subtitle string // year, episode info, etc.
	Image    *ebiten.Image
	// Set by the grid during layout
	X, Y float64
}

// PosterGrid is a horizontally scrolling row of poster items.
type PosterGrid struct {
	Items   []GridItem
	Focused int
	Label   string
	OffsetX float64
	targetOffsetX float64

	Active bool // whether this row currently has focus
}

func NewPosterGrid(label string) *PosterGrid {
	return &PosterGrid{
		Label: label,
	}
}

func (pg *PosterGrid) Update(dir Direction) (consumed bool) {
	if len(pg.Items) == 0 {
		return false
	}
	switch dir {
	case DirLeft:
		if pg.Focused > 0 {
			pg.Focused--
			pg.ensureVisible()
			return true
		}
	case DirRight:
		if pg.Focused < len(pg.Items)-1 {
			pg.Focused++
			pg.ensureVisible()
			return true
		}
	}
	return false
}

func (pg *PosterGrid) ensureVisible() {
	// Scroll to keep focused item visible
	itemX := float64(pg.Focused) * (PosterWidth + PosterGap)
	viewWidth := float64(ScreenWidth) - SectionPadding*2

	if itemX+PosterWidth-pg.targetOffsetX > viewWidth {
		pg.targetOffsetX = itemX + PosterWidth - viewWidth + PosterGap
	}
	if itemX-pg.targetOffsetX < 0 {
		pg.targetOffsetX = itemX
	}
}

func (pg *PosterGrid) AnimateScroll() {
	pg.OffsetX = Lerp(pg.OffsetX, pg.targetOffsetX, ScrollAnimSpeed)
}

func (pg *PosterGrid) Draw(dst *ebiten.Image, baseX, baseY float64) float64 {
	pg.AnimateScroll()

	// Section label
	DrawText(dst, pg.Label, baseX, baseY, FontSizeHeading, ColorText)
	baseY += SectionTitleH

	// Create a clipping sub-image for the poster row
	rowHeight := float64(PosterHeight + FontSizeSmall + 16 + PosterFocusPad*2)

	for i := range pg.Items {
		item := &pg.Items[i]
		ix := baseX + float64(i)*(PosterWidth+PosterGap) - pg.OffsetX
		iy := baseY + PosterFocusPad

		// Skip offscreen items
		if ix+PosterWidth < baseX-PosterGap || ix > float64(ScreenWidth) {
			continue
		}

		item.X = ix
		item.Y = iy

		isFocused := pg.Active && i == pg.Focused

		// Focus highlight
		if isFocused {
			vector.DrawFilledRect(dst,
				float32(ix-PosterFocusPad), float32(iy-PosterFocusPad),
				float32(PosterWidth+PosterFocusPad*2), float32(PosterHeight+PosterFocusPad*2),
				ColorFocusBorder, false)
		}

		// Poster image or placeholder
		if item.Image != nil {
			op := &ebiten.DrawImageOptions{}
			bounds := item.Image.Bounds()
			scaleX := float64(PosterWidth) / float64(bounds.Dx())
			scaleY := float64(PosterHeight) / float64(bounds.Dy())
			op.GeoM.Scale(scaleX, scaleY)
			op.GeoM.Translate(ix, iy)
			dst.DrawImage(item.Image, op)
		} else {
			// Placeholder
			vector.DrawFilledRect(dst, float32(ix), float32(iy),
				float32(PosterWidth), float32(PosterHeight),
				ColorSurface, false)
			DrawTextCentered(dst, item.Title,
				ix+PosterWidth/2, iy+PosterHeight/2,
				FontSizeSmall, ColorTextMuted)
		}

		// Title below poster
		titleColor := ColorTextSecondary
		if isFocused {
			titleColor = ColorText
		}
		title := truncateText(item.Title, PosterWidth, FontSizeCaption)
		DrawText(dst, title, ix, iy+PosterHeight+4, FontSizeCaption, titleColor)
	}

	return rowHeight + SectionTitleH
}

func (pg *PosterGrid) SelectedItem() *GridItem {
	if len(pg.Items) == 0 || pg.Focused >= len(pg.Items) {
		return nil
	}
	return &pg.Items[pg.Focused]
}

func truncateText(s string, maxWidth float64, fontSize float64) string {
	w, _ := MeasureText(s, fontSize)
	if w <= maxWidth {
		return s
	}
	for i := len(s) - 1; i > 0; i-- {
		candidate := s[:i] + "…"
		w, _ = MeasureText(candidate, fontSize)
		if w <= maxWidth {
			return candidate
		}
	}
	return "…"
}

// DrawFilledRoundRect draws a filled rectangle with rounded corners.
func DrawFilledRoundRect(dst *ebiten.Image, x, y, w, h, radius float32, clr color.Color) {
	// Ebitengine v2 vector doesn't have native round rect, so use regular rect
	vector.DrawFilledRect(dst, x, y, w, h, clr, false)
}

// CreatePlaceholderImage creates a solid color placeholder image.
func CreatePlaceholderImage(w, h int, clr color.Color) *ebiten.Image {
	img := ebiten.NewImage(w, h)
	img.Fill(clr)
	return img
}

// ImageFromRGBA converts a standard image.RGBA to an ebiten image.
func ImageFromRGBA(rgba *image.RGBA) *ebiten.Image {
	return ebiten.NewImageFromImage(rgba)
}
