package ui

import (
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

	fieldIndex int // 0=server, 1=user, 2=pass, 3=connect button
	fields     [3]*string
	labels     [3]string

	OnLogin func(server, user, pass string)
}

func NewLoginScreen(serverURL string, onLogin func(server, user, pass string)) *LoginScreen {
	ls := &LoginScreen{
		ServerURL: serverURL,
		OnLogin:   onLogin,
	}
	ls.fields = [3]*string{&ls.ServerURL, &ls.Username, &ls.Password}
	ls.labels = [3]string{"Server URL", "Username", "Password"}
	return ls
}

func (ls *LoginScreen) Name() string { return "Login" }
func (ls *LoginScreen) OnEnter()     {}
func (ls *LoginScreen) OnExit()      {}

func (ls *LoginScreen) Update() (*ScreenTransition, error) {
	// Handle text input for the focused field
	if ls.fieldIndex < 3 {
		ls.handleTextInput(ls.fields[ls.fieldIndex])
	}

	// Navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) || inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		ls.fieldIndex = (ls.fieldIndex + 1) % 4
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		ls.fieldIndex--
		if ls.fieldIndex < 0 {
			ls.fieldIndex = 3
		}
	}

	// Submit
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if ls.fieldIndex == 3 || (ls.ServerURL != "" && ls.Username != "") {
			if ls.OnLogin != nil {
				ls.OnLogin(ls.ServerURL, ls.Username, ls.Password)
			}
		}
	}

	return nil, nil
}

func (ls *LoginScreen) handleTextInput(field *string) {
	// Get typed characters
	runes := ebiten.AppendInputChars(nil)
	if len(runes) > 0 {
		*field += string(runes)
	}

	// Backspace
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(*field) > 0 {
		_, size := utf8.DecodeLastRuneInString(*field)
		*field = (*field)[:len(*field)-size]
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
		value := *ls.fields[i]
		if i == 2 && value != "" {
			value = strings.Repeat("•", utf8.RuneCountInString(value))
		}
		if value == "" && i != ls.fieldIndex {
			DrawText(dst, ls.placeholders()[i], float64(fx+10), float64(fy+12), FontSizeBody, ColorTextMuted)
		} else {
			displayVal := value
			if i == ls.fieldIndex {
				displayVal += "│" // cursor
			}
			DrawText(dst, displayVal, float64(fx+10), float64(fy+12), FontSizeBody, ColorText)
		}
	}

	// Connect button
	btnY := startY + 3*70
	btnW := fieldW
	btnH := float32(48)
	bx := float32(cx) - btnW/2

	btnColor := ColorPrimary
	if ls.fieldIndex == 3 {
		btnColor = ColorPrimaryDark
	}
	vector.DrawFilledRect(dst, bx, btnY, btnW, btnH, btnColor, false)
	if ls.fieldIndex == 3 {
		vector.StrokeRect(dst, bx, btnY, btnW, btnH, 2, ColorFocusBorder, false)
	}
	DrawTextCentered(dst, "Connect", cx, float64(btnY+btnH/2), FontSizeBody, ColorText)

	// Error message
	if ls.Error != "" {
		DrawTextCentered(dst, ls.Error, cx, float64(btnY+btnH+30), FontSizeBody, ColorError)
	}
}

func (ls *LoginScreen) placeholders() [3]string {
	return [3]string{"https://jellyfin.example.com", "username", "password"}
}
