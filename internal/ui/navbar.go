package ui

import (
	"image/color"
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

// navBtn describes a right-side navigation button.
type navBtn struct {
	id    string
	label string
}

// NavBar is a persistent navigation bar drawn at the top of every screen (except Login).
type NavBar struct {
	LibraryViews []struct{ ID, Name string }

	input        TextInput
	Active       bool
	focusSection int // 0=library buttons, 1=search bar, 2=right nav buttons
	libNavIndex  int
	navBtnIndex  int // index into rightButtons()

	ActiveScreenName string // for visual highlight of current section

	OnNavigate        func(action, id, title string) // "home", "library", "discovery", "apps", "settings"
	OnSearch          func(query string)
	JellyseerrEnabled func() bool
	WebAppsEnabled    func() bool
}

// NewNavBar creates a new NavBar.
func NewNavBar() *NavBar {
	return &NavBar{
		focusSection: 1, // default to search bar
	}
}

// rightButtons returns the dynamic list of right-side nav buttons.
func (nb *NavBar) rightButtons() []navBtn {
	var btns []navBtn
	if nb.JellyseerrEnabled != nil && nb.JellyseerrEnabled() {
		btns = append(btns, navBtn{"discovery", "Discovery"})
	}
	if nb.WebAppsEnabled != nil && nb.WebAppsEnabled() {
		btns = append(btns, navBtn{"apps", "Apps"})
	}
	btns = append(btns, navBtn{"settings", "Settings"})
	return btns
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
	case 0: // Home + Library buttons (index 0 = Home, 1+ = library views)
		sectionLen := 1 + len(nb.LibraryViews) // Home + libraries

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if nb.libNavIndex == 0 {
				if nb.OnNavigate != nil {
					nb.OnNavigate("home", "", "")
				}
			} else {
				view := nb.LibraryViews[nb.libNavIndex-1]
				if nb.OnNavigate != nil {
					nb.OnNavigate("library", view.ID, view.Name)
				}
			}
			nb.Active = false
			return NavBarActionDefocus
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if nb.libNavIndex < sectionLen-1 {
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

		// Left at start → Home + library buttons
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) && nb.input.CursorAtStart() {
			nb.libNavIndex = len(nb.LibraryViews) // last library button, or Home if no libraries
			nb.focusSection = 0
		}

		// Right at end → nav buttons
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) && nb.input.CursorAtEnd() {
			nb.navBtnIndex = 0
			nb.focusSection = 2
		}

	case 2: // Right-side nav buttons (dynamic)
		btns := nb.rightButtons()
		if nb.navBtnIndex >= len(btns) {
			nb.navBtnIndex = len(btns) - 1
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if nb.navBtnIndex >= 0 && nb.navBtnIndex < len(btns) {
				if nb.OnNavigate != nil {
					nb.OnNavigate(btns[nb.navBtnIndex].id, "", "")
				}
			}
			nb.Active = false
			return NavBarActionDefocus
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if nb.navBtnIndex < len(btns)-1 {
				nb.navBtnIndex++
			}
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if nb.navBtnIndex > 0 {
				nb.navBtnIndex--
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

	// Home button
	homeBtnX := 230.0
	homeTw, _ := MeasureText("Home", FontSizeBody)
	homeBtnW := homeTw + 28
	if PointInRect(mx, my, homeBtnX, 12, homeBtnW, 38) {
		if nb.OnNavigate != nil {
			nb.OnNavigate("home", "", "")
		}
		return true
	}

	// Library buttons
	libBtnX := homeBtnX + homeBtnW + 10
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

	// Right-side buttons (laid out right-to-left)
	btns := nb.rightButtons()
	btnX := float64(ScreenWidth) - SectionPadding
	for i := len(btns) - 1; i >= 0; i-- {
		bw := nb.rightBtnWidth(btns[i])
		btnX -= bw
		if PointInRect(mx, my, btnX, 12, bw, 38) {
			if nb.OnNavigate != nil {
				nb.OnNavigate(btns[i].id, "", "")
			}
			return true
		}
		btnX -= 10
	}

	return false
}

// rightBtnWidth returns the draw width for a right-side button.
func (nb *NavBar) rightBtnWidth(btn navBtn) float64 {
	tw, _ := MeasureText(btn.label, FontSizeBody)
	// Buttons with icons (discovery, settings) get extra space for the icon
	if btn.id == "discovery" || btn.id == "settings" {
		return tw + 44 // 16px icon area + padding
	}
	return tw + 28
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

	// Home button
	homeBtnX := 230.0
	{
		tw, _ := MeasureText("Home", FontSizeBody)
		btnW := tw + 28
		btnH := 38.0
		btnY := 12.0
		focused := nb.Active && nb.focusSection == 0 && nb.libNavIndex == 0
		active := nb.ActiveScreenName == "Home"

		if focused {
			vector.DrawFilledRect(dst, float32(homeBtnX), float32(btnY), float32(btnW), float32(btnH), ColorPrimary, false)
			DrawTextCentered(dst, "Home", homeBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorBackground)
		} else if active {
			vector.DrawFilledRect(dst, float32(homeBtnX), float32(btnY), float32(btnW), float32(btnH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(homeBtnX), float32(btnY), float32(btnW), float32(btnH), 2, ColorPrimary, false)
			DrawTextCentered(dst, "Home", homeBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorText)
		} else {
			vector.DrawFilledRect(dst, float32(homeBtnX), float32(btnY), float32(btnW), float32(btnH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(homeBtnX), float32(btnY), float32(btnW), float32(btnH), 1, ColorPrimary, false)
			DrawTextCentered(dst, "Home", homeBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorText)
		}
		homeBtnX += btnW + 10
	}

	// Library nav buttons
	libBtnX := homeBtnX
	for i, view := range nb.LibraryViews {
		tw, _ := MeasureText(view.Name, FontSizeBody)
		btnW := tw + 28
		btnH := 38.0
		btnY := 12.0

		focused := nb.Active && nb.focusSection == 0 && i+1 == nb.libNavIndex
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

	// Right-side buttons (laid out right-to-left)
	btns := nb.rightButtons()
	btnX := float64(ScreenWidth) - SectionPadding
	for i := len(btns) - 1; i >= 0; i-- {
		btn := btns[i]
		bw := nb.rightBtnWidth(btn)
		btnX -= bw
		nb.drawRightButton(dst, btn, btnX, i)
		btnX -= 10
	}
}

// drawRightButton draws a single right-side nav button.
func (nb *NavBar) drawRightButton(dst *ebiten.Image, btn navBtn, x float64, idx int) {
	w := nb.rightBtnWidth(btn)
	h := 38.0
	y := 12.0
	focused := nb.Active && nb.focusSection == 2 && nb.navBtnIndex == idx
	active := nb.ActiveScreenName == btn.label || (btn.id == "discovery" && nb.ActiveScreenName == "Discovery")

	hasIcon := btn.id == "discovery" || btn.id == "settings"
	textOffsetX := 0.0
	if hasIcon {
		textOffsetX = 8
	}

	// Determine border color for this button type
	borderColor := ColorPrimary
	if btn.id == "settings" {
		borderColor = ColorTextSecondary
	}

	if focused {
		vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), ColorPrimary, false)
		DrawTextCentered(dst, btn.label, x+w/2+textOffsetX, y+h/2, FontSizeBody, ColorBackground)
		if hasIcon {
			nb.drawBtnIcon(dst, btn.id, float32(x+16), float32(y+h/2), 7, ColorBackground)
		}
	} else if active {
		vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 2, borderColor, false)
		DrawTextCentered(dst, btn.label, x+w/2+textOffsetX, y+h/2, FontSizeBody, ColorText)
		if hasIcon {
			nb.drawBtnIcon(dst, btn.id, float32(x+16), float32(y+h/2), 7, borderColor)
		}
	} else {
		vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 1, borderColor, false)
		DrawTextCentered(dst, btn.label, x+w/2+textOffsetX, y+h/2, FontSizeBody, ColorText)
		if hasIcon {
			nb.drawBtnIcon(dst, btn.id, float32(x+16), float32(y+h/2), 7, borderColor)
		}
	}
}

// drawBtnIcon draws the icon for a specific button type.
func (nb *NavBar) drawBtnIcon(dst *ebiten.Image, id string, cx, cy, radius float32, clr color.Color) {
	switch id {
	case "discovery":
		drawCompassIcon(dst, cx, cy, radius, clr)
	case "settings":
		drawGearIcon(dst, cx, cy, radius, clr)
	}
}
