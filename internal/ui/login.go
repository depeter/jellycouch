package ui

import (
	"image/color"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// LoginScreen handles server URL and credentials input.
type LoginScreen struct {
	ServerURL string
	Username  string
	Password  string
	Error     string
	Busy      bool

	fieldIndex int // 0=server, 1=user, 2=pass, 3=connect button
	inputs     [3]TextInput
	labels     [3]string

	OnLogin func(screen *LoginScreen, server, user, pass string)
}

func NewLoginScreen(serverURL string, onLogin func(screen *LoginScreen, server, user, pass string)) *LoginScreen {
	ls := &LoginScreen{
		ServerURL: serverURL,
		OnLogin:   onLogin,
	}
	ls.inputs = [3]TextInput{
		NewTextInput(serverURL),
		NewTextInput(""),
		NewTextInput(""),
	}
	ls.labels = [3]string{"Server URL", "Username", "Password"}
	return ls
}

func (ls *LoginScreen) Name() string { return "Login" }
func (ls *LoginScreen) OnEnter()     {}
func (ls *LoginScreen) OnExit()      {}

func (ls *LoginScreen) Update() (*ScreenTransition, error) {
	if ls.Busy {
		return nil, nil
	}

	// Handle text input for the focused field
	if ls.fieldIndex < 3 {
		ls.inputs[ls.fieldIndex].Update()
		// Sync back to exported fields
		ls.ServerURL = ls.inputs[0].Text
		ls.Username = ls.inputs[1].Text
		ls.Password = ls.inputs[2].Text
	}

	// Mouse click — check each field and the button
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		ls.handleClick(float64(mx), float64(my))
	}

	// Navigation: Tab / Shift+Tab / Arrow keys (only Up/Down for field nav)
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			ls.fieldIndex--
			if ls.fieldIndex < 0 {
				ls.fieldIndex = 3
			}
		} else {
			ls.fieldIndex = (ls.fieldIndex + 1) % 4
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		ls.fieldIndex = (ls.fieldIndex + 1) % 4
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		ls.fieldIndex--
		if ls.fieldIndex < 0 {
			ls.fieldIndex = 3
		}
	}

	// Submit — Enter from any field or the button
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		ls.submit()
	}

	return nil, nil
}

func (ls *LoginScreen) submit() {
	server := strings.TrimSpace(ls.ServerURL)
	user := strings.TrimSpace(ls.Username)
	pass := ls.Password

	if server == "" {
		ls.Error = "Server URL is required"
		ls.fieldIndex = 0
		return
	}
	if user == "" {
		ls.Error = "Username is required"
		ls.fieldIndex = 1
		return
	}

	ls.Error = ""
	if ls.OnLogin != nil {
		ls.OnLogin(ls, server, user, pass)
	}
}

func (ls *LoginScreen) handleClick(mx, my float64) {
	cx := float64(ScreenWidth) / 2
	cy := float64(ScreenHeight)/2 - 100
	fieldW := 400.0
	fieldH := 44.0
	startY := cy

	// Check text fields
	for i := 0; i < 3; i++ {
		fy := startY + float64(i)*70
		fx := cx - fieldW/2
		if mx >= fx && mx <= fx+fieldW && my >= fy && my <= fy+fieldH {
			ls.fieldIndex = i
			return
		}
	}

	// Check connect button
	btnY := startY + 3*70
	btnH := 48.0
	bx := cx - fieldW/2
	if mx >= bx && mx <= bx+fieldW && my >= btnY && my <= btnY+btnH {
		ls.fieldIndex = 3
		ls.submit()
	}
}

func (ls *LoginScreen) Draw(dst *ebiten.Image) {
	dst.Fill(ColorBackground)

	cx := float64(ScreenWidth) / 2
	cy := float64(ScreenHeight)/2 - 100

	// Title
	DrawTextCentered(dst, "JellyCouch", cx, cy-80, FontSizeTitle+8, ColorPrimary)
	DrawTextCentered(dst, "Connect to your Jellyfin server", cx, cy-40, FontSizeBody, ColorTextSecondary)

	// Fields
	fieldW := float32(400)
	fieldH := float32(44)
	startY := float32(cy)

	for i := 0; i < 3; i++ {
		fy := startY + float32(i)*70
		fx := float32(cx) - fieldW/2

		// Label
		DrawText(dst, ls.labels[i], float64(fx), float64(fy-20), FontSizeSmall, ColorTextSecondary)

		// Field background
		bgColor := ColorSurface
		if i == ls.fieldIndex {
			bgColor = ColorSurfaceHover
		}
		vector.DrawFilledRect(dst, fx, fy, fieldW, fieldH, bgColor, false)

		// Focus border
		if i == ls.fieldIndex {
			vector.StrokeRect(dst, fx, fy, fieldW, fieldH, 2, ColorFocusBorder, false)
		}

		// Field text
		value := ls.inputs[i].Text
		if i == 2 && value != "" {
			// Password masking — show dots but preserve cursor position
			masked := strings.Repeat("•", utf8.RuneCountInString(value))
			if i == ls.fieldIndex {
				// Insert cursor into masked text at same position
				before := strings.Repeat("•", ls.inputs[i].Cursor)
				after := strings.Repeat("•", utf8.RuneCountInString(value)-ls.inputs[i].Cursor)
				value = before + "│" + after
			} else {
				value = masked
			}
		} else if i == ls.fieldIndex {
			value = ls.inputs[i].DisplayText()
		}

		if ls.inputs[i].Text == "" && i != ls.fieldIndex {
			DrawText(dst, ls.placeholders()[i], float64(fx+10), float64(fy+12), FontSizeBody, ColorTextMuted)
		} else {
			DrawText(dst, value, float64(fx+10), float64(fy+12), FontSizeBody, ColorText)
		}
	}

	// Connect button
	btnY := startY + 3*70
	btnW := fieldW
	btnH := float32(48)
	bx := float32(cx) - btnW/2

	var btnColor color.Color = ColorPrimary
	if ls.fieldIndex == 3 {
		btnColor = ColorPrimaryDark
	}
	if ls.Busy {
		btnColor = ColorSurface
	}
	vector.DrawFilledRect(dst, bx, btnY, btnW, btnH, btnColor, false)
	if ls.fieldIndex == 3 {
		vector.StrokeRect(dst, bx, btnY, btnW, btnH, 2, ColorFocusBorder, false)
	}

	btnLabel := "Connect"
	if ls.Busy {
		btnLabel = "Connecting..."
	}
	DrawTextCentered(dst, btnLabel, cx, float64(btnY+btnH/2), FontSizeBody, ColorText)

	// Error message
	if ls.Error != "" {
		DrawTextCentered(dst, ls.Error, cx, float64(btnY+btnH+30), FontSizeBody, ColorError)
	}

	// Hint
	DrawTextCentered(dst, "Tab to navigate, Enter to submit",
		cx, float64(ScreenHeight)-40, FontSizeSmall, ColorTextMuted)
}

func (ls *LoginScreen) placeholders() [3]string {
	return [3]string{"https://jellyfin.example.com", "username", "password"}
}
