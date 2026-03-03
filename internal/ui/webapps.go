package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/config"
)

// WebAppsScreen displays a vertical list of configured web apps.
type WebAppsScreen struct {
	apps       []config.WebApp
	focusIndex int
	OnLaunch   func(url string)
}

// NewWebAppsScreen creates a screen listing the given web apps.
func NewWebAppsScreen(apps []config.WebApp) *WebAppsScreen {
	return &WebAppsScreen{apps: apps}
}

func (s *WebAppsScreen) Name() string { return "Apps" }
func (s *WebAppsScreen) OnEnter()     {}
func (s *WebAppsScreen) OnExit()      {}

func (s *WebAppsScreen) Update() (*ScreenTransition, error) {
	dir, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	if dir == DirUp {
		if s.focusIndex > 0 {
			s.focusIndex--
		} else {
			return &ScreenTransition{Type: TransitionFocusNavBar}, nil
		}
	}
	if dir == DirDown {
		if s.focusIndex < len(s.apps)-1 {
			s.focusIndex++
		}
	}

	if enter && len(s.apps) > 0 && s.OnLaunch != nil {
		s.OnLaunch(s.apps[s.focusIndex].URL)
	}

	// Mouse click
	if mx, my, clicked := MouseJustClicked(); clicked {
		for i := range s.apps {
			rowY := NavBarHeight + 60 + float64(i)*60
			if PointInRect(mx, my, SectionPadding, rowY, float64(ScreenWidth)-SectionPadding*2, 50) {
				s.focusIndex = i
				if s.OnLaunch != nil {
					s.OnLaunch(s.apps[i].URL)
				}
			}
		}
	}

	return nil, nil
}

func (s *WebAppsScreen) Draw(dst *ebiten.Image) {
	// Title
	DrawText(dst, "Web Apps", SectionPadding, NavBarHeight+20, FontSizeTitle, ColorText)

	if len(s.apps) == 0 {
		DrawText(dst, "No web apps available.",
			SectionPadding, NavBarHeight+80, FontSizeBody, ColorTextMuted)
		return
	}

	for i, app := range s.apps {
		rowY := NavBarHeight + 60 + float64(i)*60
		rowW := float64(ScreenWidth) - SectionPadding*2
		rowH := 50.0
		focused := i == s.focusIndex

		if focused {
			vector.DrawFilledRect(dst, float32(SectionPadding), float32(rowY),
				float32(rowW), float32(rowH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(SectionPadding), float32(rowY),
				float32(rowW), float32(rowH), 2, ColorPrimary, false)
		} else {
			vector.DrawFilledRect(dst, float32(SectionPadding), float32(rowY),
				float32(rowW), float32(rowH), ColorSurface, false)
		}

		// App name
		DrawText(dst, app.Name, SectionPadding+16, rowY+10, FontSizeBody, ColorText)
		// URL in muted color
		DrawText(dst, app.URL, SectionPadding+16, rowY+30, FontSizeSmall, ColorTextMuted)
	}
}
