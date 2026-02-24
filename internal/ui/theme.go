package ui

import "image/color"

// Colors — dark theme inspired by Jellyfin branding
var (
	ColorBackground    = color.RGBA{R: 0x10, G: 0x10, B: 0x14, A: 0xFF}
	ColorSurface       = color.RGBA{R: 0x1C, G: 0x1C, B: 0x24, A: 0xFF}
	ColorSurfaceHover  = color.RGBA{R: 0x28, G: 0x28, B: 0x34, A: 0xFF}
	ColorPrimary       = color.RGBA{R: 0x00, G: 0xA4, B: 0xDC, A: 0xFF} // Jellyfin blue
	ColorPrimaryDark   = color.RGBA{R: 0x00, G: 0x78, B: 0xA8, A: 0xFF}
	ColorAccent        = color.RGBA{R: 0xAA, G: 0x5C, B: 0xC3, A: 0xFF} // Purple accent
	ColorText          = color.RGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	ColorTextSecondary = color.RGBA{R: 0x90, G: 0x90, B: 0x9C, A: 0xFF}
	ColorTextMuted     = color.RGBA{R: 0x60, G: 0x60, B: 0x6C, A: 0xFF}
	ColorFocusBorder   = color.RGBA{R: 0x00, G: 0xA4, B: 0xDC, A: 0xFF}
	ColorOverlay       = color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xC0}
	ColorError         = color.RGBA{R: 0xE0, G: 0x40, B: 0x40, A: 0xFF}
	ColorSuccess       = color.RGBA{R: 0x40, G: 0xC0, B: 0x60, A: 0xFF}
)

// Layout constants
const (
	PosterWidth     = 150
	PosterHeight    = 225
	PosterGap       = 20
	PosterFocusPad  = 6

	BackdropHeight  = 400

	SectionPadding  = 40
	SectionGap      = 30
	SectionTitleH   = 36

	NavBarHeight    = 60
	NavBarPadding   = 20

	FontSizeTitle   = 28
	FontSizeHeading = 22
	FontSizeBody    = 16
	FontSizeSmall   = 13
	FontSizeCaption = 11

	FocusAnimSpeed  = 0.15
	ScrollAnimSpeed = 0.12

	ScreenWidth     = 1920
	ScreenHeight    = 1080
)
