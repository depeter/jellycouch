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
	Progress float64 // 0.0 to 1.0, playback progress
	Watched  bool
	// Jellyseerr request status: 0=none, 2=pending, 3=partial, 4=processing, 5=available
	RequestStatus int
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

// HandleClick checks if (mx, my) hits any item and returns its index.
func (pg *PosterGrid) HandleClick(mx, my int) (clickedIndex int, ok bool) {
	for i := range pg.Items {
		item := &pg.Items[i]
		if PointInRect(mx, my, item.X, item.Y, PosterWidth, PosterHeight) {
			return i, true
		}
	}
	return -1, false
}

func (pg *PosterGrid) Draw(dst *ebiten.Image, baseX, baseY float64) float64 {
	pg.AnimateScroll()

	// Section label
	DrawText(dst, pg.Label, baseX, baseY, FontSizeHeading, ColorText)
	baseY += SectionTitleH

	// Create a clipping sub-image for the poster row
	rowHeight := float64(PosterHeight + FontSizeSmall + 16 + PosterFocusPad*2)

	hasLeft := pg.OffsetX > 1
	hasRight := false

	for i := range pg.Items {
		item := &pg.Items[i]
		ix := baseX + float64(i)*(PosterWidth+PosterGap) - pg.OffsetX
		iy := baseY + PosterFocusPad

		// Skip offscreen items
		if ix+PosterWidth < baseX-PosterGap || ix > float64(ScreenWidth) {
			if ix > float64(ScreenWidth) {
				hasRight = true
			}
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
			DrawImageCover(dst, item.Image, ix, iy, PosterWidth, PosterHeight)
		} else {
			// Placeholder
			vector.DrawFilledRect(dst, float32(ix), float32(iy),
				float32(PosterWidth), float32(PosterHeight),
				ColorSurface, false)
			DrawTextCentered(dst, item.Title,
				ix+PosterWidth/2, iy+PosterHeight/2,
				FontSizeSmall, ColorTextMuted)
		}

		// Progress bar at bottom of poster
		if item.Progress > 0 && item.Progress < 1.0 {
			barH := float32(4)
			barY := float32(iy + PosterHeight - float64(barH))
			// Background
			vector.DrawFilledRect(dst, float32(ix), barY,
				float32(PosterWidth), barH,
				color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x80}, false)
			// Progress fill
			vector.DrawFilledRect(dst, float32(ix), barY,
				float32(float64(PosterWidth)*item.Progress), barH,
				ColorPrimary, false)
		}

		// Watched checkmark badge (top-right corner)
		if item.Watched {
			badgeR := float32(10)
			badgeCX := float32(ix+PosterWidth) - badgeR - 4
			badgeCY := float32(iy) + badgeR + 4
			// Green circle background
			vector.DrawFilledCircle(dst, badgeCX, badgeCY, badgeR, ColorSuccess, false)
			// Checkmark (simple "✓" text centered)
			DrawTextCentered(dst, "✓", float64(badgeCX), float64(badgeCY), FontSizeSmall, ColorText)
		}

		// Request status badge (full-width banner at bottom of poster)
		if item.RequestStatus > 0 && item.Progress == 0 {
			drawRequestBadge(dst, item.RequestStatus, ix, iy)
		}

		// Title below poster
		titleColor := ColorTextSecondary
		if isFocused {
			titleColor = ColorText
		}
		title := truncateText(item.Title, PosterWidth, FontSizeCaption)
		DrawText(dst, title, ix, iy+PosterHeight+4, FontSizeCaption, titleColor)
	}

	// Check if last item extends beyond view
	if len(pg.Items) > 0 {
		lastX := baseX + float64(len(pg.Items)-1)*(PosterWidth+PosterGap) - pg.OffsetX
		if lastX+PosterWidth > float64(ScreenWidth) {
			hasRight = true
		}
	}

	// Scroll edge indicators
	indicatorY := baseY + PosterFocusPad + PosterHeight/2
	if hasLeft {
		DrawTextCentered(dst, "◀", baseX-10, indicatorY, FontSizeBody, ColorTextMuted)
	}
	if hasRight {
		DrawTextCentered(dst, "▶", float64(ScreenWidth)-SectionPadding+10, indicatorY, FontSizeBody, ColorTextMuted)
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

// DrawImageCover draws an image scaled to cover the target rect, preserving aspect ratio and center-cropping.
func DrawImageCover(dst *ebiten.Image, src *ebiten.Image, x, y, w, h float64) {
	bounds := src.Bounds()
	srcW := float64(bounds.Dx())
	srcH := float64(bounds.Dy())

	// Scale uniformly to cover the target area
	scale := w / srcW
	if h/srcH > scale {
		scale = h / srcH
	}

	scaledW := srcW * scale
	scaledH := srcH * scale

	// Center offset (crop equally from both sides)
	offsetX := (scaledW - w) / 2
	offsetY := (scaledH - h) / 2

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x-offsetX, y-offsetY)
	op.Filter = ebiten.FilterLinear

	// Clip to target rect using a sub-image of the destination
	// Since Ebitengine doesn't have clip regions, we use SubImage on dst
	clipRect := image.Rect(int(x), int(y), int(x+w), int(y+h))
	sub := dst.SubImage(clipRect).(*ebiten.Image)
	sub.DrawImage(src, op)
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

// drawRequestBadge draws a full-width status banner at the bottom of a poster.
func drawRequestBadge(dst *ebiten.Image, status int, x, y float64) {
	label := ""
	switch status {
	case 2: // pending
		label = "Pending"
	case 3: // partially available
		label = "Partial"
	case 4: // processing
		label = "Processing"
	case 5: // available
		label = "Available"
	default:
		return
	}
	badgeColor := statusBadgeColor(status)
	bh := FontSizeSmall + 8.0
	bannerY := y + PosterHeight - bh
	vector.DrawFilledRect(dst, float32(x), float32(bannerY),
		float32(PosterWidth), float32(bh), badgeColor, false)
	DrawTextCentered(dst, label, x+PosterWidth/2, bannerY+bh/2, FontSizeSmall, ColorText)
}

// statusBadgeColor returns the badge background color for a media status.
func statusBadgeColor(status int) color.RGBA {
	switch status {
	case 2: // pending
		return color.RGBA{R: 0xCC, G: 0xA0, B: 0x00, A: 0xE0} // yellow
	case 3: // partially available
		return color.RGBA{R: 0x00, G: 0x80, B: 0xCC, A: 0xE0} // blue
	case 4: // processing
		return color.RGBA{R: 0x00, G: 0x80, B: 0xCC, A: 0xE0} // blue
	case 5: // available
		return color.RGBA{R: 0x30, G: 0xA0, B: 0x50, A: 0xE0} // green
	default:
		return color.RGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xE0} // gray
	}
}
