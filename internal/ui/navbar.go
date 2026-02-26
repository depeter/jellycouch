package ui

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// NavBarAction represents the result of a navbar Update cycle.
type NavBarAction int

const (
	NavBarActionNone    NavBarAction = iota
	NavBarActionDefocus              // return focus to screen below
)

// NavBar is a persistent navigation bar drawn at the top of every screen (except Login).
type NavBar struct {
	LibraryViews []struct{ ID, Name string }

	input        TextInput
	Active       bool
	focusSection int // 0=library buttons, 1=search bar, 2=right nav buttons
	libNavIndex  int
	navBtnIndex  int // 0=discovery, 1=settings

	ActiveScreenName string // for visual highlight of current section

	OnNavigate        func(action, id, title string) // "home", "library", "discovery", "settings"
	OnSearch          func(query string)
	JellyseerrEnabled func() bool
}

// NewNavBar creates a new NavBar.
func NewNavBar() *NavBar {
	return &NavBar{
		focusSection: 1, // default to search bar
	}
}

// FocusFromBelow activates keyboard focus on the navbar (called when screen hands focus up).
func (nb *NavBar) FocusFromBelow() {
	nb.Active = true
	nb.focusSection = 1 // start at search bar
}

// Update processes keyboard input when the navbar is active. Returns an action.
func (nb *NavBar) Update() NavBarAction {
	if !nb.Active {
		return NavBarActionNone
	}

	// Down or Escape returns focus to the screen
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		nb.Active = false
		return NavBarActionDefocus
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		nb.Active = false
		return NavBarActionDefocus
	}

	switch nb.focusSection {
	case 0: // Library buttons
		if len(nb.LibraryViews) == 0 {
			nb.focusSection = 1
			return NavBarActionNone
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			view := nb.LibraryViews[nb.libNavIndex]
			if nb.OnNavigate != nil {
				nb.OnNavigate("library", view.ID, view.Name)
			}
			nb.Active = false
			return NavBarActionDefocus
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if nb.libNavIndex < len(nb.LibraryViews)-1 {
				nb.libNavIndex++
			} else {
				nb.focusSection = 1
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if nb.libNavIndex > 0 {
				nb.libNavIndex--
			}
		}

	case 1: // Search bar
		nb.input.Update()

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && nb.input.Text != "" {
			query := nb.input.Text
			nb.input.Clear()
			if nb.OnSearch != nil {
				nb.OnSearch(query)
			}
			nb.Active = false
			return NavBarActionDefocus
		}

		// Left at start → library buttons
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) && nb.input.CursorAtStart() && len(nb.LibraryViews) > 0 {
			nb.libNavIndex = len(nb.LibraryViews) - 1
			nb.focusSection = 0
		}

		// Right at end → nav buttons
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) && nb.input.CursorAtEnd() {
			hasDiscovery := nb.JellyseerrEnabled != nil && nb.JellyseerrEnabled()
			if hasDiscovery {
				nb.navBtnIndex = 0
			} else {
				nb.navBtnIndex = 1
			}
			nb.focusSection = 2
		}

	case 2: // Nav buttons (discovery/settings)
		hasDiscovery := nb.JellyseerrEnabled != nil && nb.JellyseerrEnabled()

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if nb.navBtnIndex == 0 && hasDiscovery {
				if nb.OnNavigate != nil {
					nb.OnNavigate("discovery", "", "")
				}
			} else {
				if nb.OnNavigate != nil {
					nb.OnNavigate("settings", "", "")
				}
			}
			nb.Active = false
			return NavBarActionDefocus
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if nb.navBtnIndex == 0 && hasDiscovery {
				nb.navBtnIndex = 1
			}
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if nb.navBtnIndex == 1 && hasDiscovery {
				nb.navBtnIndex = 0
			} else {
				nb.focusSection = 1 // back to search
			}
		}
	}

	return NavBarActionNone
}

// HandleClick checks if (mx, my) hits a navbar element and triggers navigation. Returns true if consumed.
func (nb *NavBar) HandleClick(mx, my int) bool {
	if float64(my) >= NavBarHeight {
		return false
	}

	// JellyCouch title → home
	if PointInRect(mx, my, SectionPadding, 12, 180, 38) {
		if nb.OnNavigate != nil {
			nb.OnNavigate("home", "", "")
		}
		return true
	}

	// Library buttons
	libBtnX := 230.0
	for _, view := range nb.LibraryViews {
		tw, _ := MeasureText(view.Name, FontSizeBody)
		btnW := tw + 28
		if PointInRect(mx, my, libBtnX, 12, btnW, 38) {
			if nb.OnNavigate != nil {
				nb.OnNavigate("library", view.ID, view.Name)
			}
			return true
		}
		libBtnX += btnW + 10
	}

	// Search bar
	searchX := float64(ScreenWidth)/2 - 200
	if PointInRect(mx, my, searchX, 12, 400, 38) {
		nb.Active = true
		nb.focusSection = 1
		return true
	}

	// Settings button
	settingsX := float64(ScreenWidth) - SectionPadding - 100
	if PointInRect(mx, my, settingsX, 12, 100, 38) {
		if nb.OnNavigate != nil {
			nb.OnNavigate("settings", "", "")
		}
		return true
	}

	// Discovery button
	if nb.JellyseerrEnabled != nil && nb.JellyseerrEnabled() {
		reqX := settingsX - 120
		if PointInRect(mx, my, reqX, 12, 110, 38) {
			if nb.OnNavigate != nil {
				nb.OnNavigate("discovery", "", "")
			}
			return true
		}
	}

	return false
}

// Draw renders the navbar overlay.
func (nb *NavBar) Draw(dst *ebiten.Image) {
	// Solid background bar
	vector.DrawFilledRect(dst, 0, 0, float32(ScreenWidth), float32(NavBarHeight), ColorBackground, false)
	// Bottom separator line
	vector.DrawFilledRect(dst, 0, float32(NavBarHeight-1), float32(ScreenWidth), 1, ColorSurfaceHover, false)

	// JellyCouch title (clickable home)
	homeColor := ColorPrimary
	if nb.ActiveScreenName == "Home" {
		homeColor = ColorText
	}
	DrawText(dst, "JellyCouch", SectionPadding, 16, FontSizeTitle, homeColor)

	// Library nav buttons
	libBtnX := 230.0
	for i, view := range nb.LibraryViews {
		tw, _ := MeasureText(view.Name, FontSizeBody)
		btnW := tw + 28
		btnH := 38.0
		btnY := 12.0

		focused := nb.Active && nb.focusSection == 0 && i == nb.libNavIndex
		active := strings.HasPrefix(nb.ActiveScreenName, "Library: "+view.Name)

		if focused {
			vector.DrawFilledRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), ColorPrimary, false)
			DrawTextCentered(dst, view.Name, libBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorBackground)
		} else if active {
			vector.DrawFilledRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), 2, ColorPrimary, false)
			DrawTextCentered(dst, view.Name, libBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorText)
		} else {
			vector.DrawFilledRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), 1, ColorPrimary, false)
			DrawTextCentered(dst, view.Name, libBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorText)
		}
		libBtnX += btnW + 10
	}

	// Search bar (center)
	searchX := float64(ScreenWidth)/2 - 200
	searchY := 12.0
	searchW := 400.0
	searchH := 38.0
	if nb.Active && nb.focusSection == 1 {
		vector.DrawFilledRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), 2, ColorFocusBorder, false)
		if nb.input.Text == "" {
			DrawText(dst, "Search...", searchX+14, searchY+10, FontSizeBody, ColorTextMuted)
		}
		DrawText(dst, nb.input.DisplayText(), searchX+14, searchY+10, FontSizeBody, ColorText)
	} else {
		vector.DrawFilledRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), ColorSurface, false)
		vector.StrokeRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), 1, ColorTextMuted, false)
		if nb.input.Text != "" {
			DrawText(dst, nb.input.Text, searchX+14, searchY+10, FontSizeBody, ColorText)
		} else {
			DrawText(dst, "Search library...", searchX+14, searchY+10, FontSizeBody, ColorTextMuted)
		}
	}

	// Right-side buttons
	settingsX := float64(ScreenWidth) - SectionPadding - 100

	// Discovery button (only when Jellyseerr configured)
	if nb.JellyseerrEnabled != nil && nb.JellyseerrEnabled() {
		reqX := settingsX - 120
		reqY := 12.0
		reqW := 110.0
		reqH := 38.0
		focused := nb.Active && nb.focusSection == 2 && nb.navBtnIndex == 0
		active := nb.ActiveScreenName == "Discovery"
		if focused {
			vector.DrawFilledRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), ColorPrimary, false)
			DrawTextCentered(dst, "Discovery", reqX+reqW/2+8, reqY+reqH/2, FontSizeBody, ColorBackground)
			drawCompassIcon(dst, float32(reqX+16), float32(reqY+reqH/2), 7, ColorBackground)
		} else if active {
			vector.DrawFilledRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), 2, ColorPrimary, false)
			DrawTextCentered(dst, "Discovery", reqX+reqW/2+8, reqY+reqH/2, FontSizeBody, ColorText)
			drawCompassIcon(dst, float32(reqX+16), float32(reqY+reqH/2), 7, ColorPrimary)
		} else {
			vector.DrawFilledRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), 1, ColorPrimary, false)
			DrawTextCentered(dst, "Discovery", reqX+reqW/2+8, reqY+reqH/2, FontSizeBody, ColorText)
			drawCompassIcon(dst, float32(reqX+16), float32(reqY+reqH/2), 7, ColorPrimary)
		}
	}

	// Settings button
	settingsY := 12.0
	settingsW := 100.0
	settingsH := 38.0
	sfocused := nb.Active && nb.focusSection == 2 && nb.navBtnIndex == 1
	sactive := nb.ActiveScreenName == "Settings"
	if sfocused {
		vector.DrawFilledRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), ColorPrimary, false)
		DrawTextCentered(dst, "Settings", settingsX+settingsW/2+8, settingsY+settingsH/2, FontSizeBody, ColorBackground)
		drawGearIcon(dst, float32(settingsX+16), float32(settingsY+settingsH/2), 7, ColorBackground)
	} else if sactive {
		vector.DrawFilledRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), 2, ColorTextSecondary, false)
		DrawTextCentered(dst, "Settings", settingsX+settingsW/2+8, settingsY+settingsH/2, FontSizeBody, ColorText)
		drawGearIcon(dst, float32(settingsX+16), float32(settingsY+settingsH/2), 7, ColorTextSecondary)
	} else {
		vector.DrawFilledRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), 1, ColorTextSecondary, false)
		DrawTextCentered(dst, "Settings", settingsX+settingsW/2+8, settingsY+settingsH/2, FontSizeBody, ColorText)
		drawGearIcon(dst, float32(settingsX+16), float32(settingsY+settingsH/2), 7, ColorTextSecondary)
	}
}
