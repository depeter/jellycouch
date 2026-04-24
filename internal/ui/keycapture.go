package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/keymap"
)

// KeyCaptureOverlay is a modal that captures the next key the user presses
// and returns its config name. Escape cancels without capturing.
type KeyCaptureOverlay struct {
	title    string
	current  string // existing binding for context in the UI
	done     bool
	result   string
	canceled bool
}

// NewKeyCaptureOverlay constructs the modal. Title is the action name
// (e.g. "Play / Pause") shown at the top; current is the existing binding
// displayed as "Currently: <name>".
func NewKeyCaptureOverlay(title, current string) *KeyCaptureOverlay {
	return &KeyCaptureOverlay{title: title, current: current}
}

// Done reports whether the overlay has resolved. canceled=true means the
// user hit Escape; otherwise result holds the captured key name (may be
// "" if the pressed key isn't remappable — callers should treat that as
// "no change").
func (kc *KeyCaptureOverlay) Done() (done bool, result string, canceled bool) {
	return kc.done, kc.result, kc.canceled
}

func (kc *KeyCaptureOverlay) Update() {
	if kc.done {
		return
	}
	// Escape cancels. Treat as canceled even though KeyEscape has a name,
	// since Escape is reserved for nav across the app.
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		kc.done = true
		kc.canceled = true
		return
	}
	// First just-pressed key that has a remappable name wins.
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		if name := keymap.Name(k); name != "" {
			kc.result = name
			kc.done = true
			return
		}
	}
}

func (kc *KeyCaptureOverlay) Draw(dst *ebiten.Image) {
	// Darken background
	vector.DrawFilledRect(dst, 0, 0, float32(ScreenWidth), float32(ScreenHeight), ColorOverlay, false)

	// Centered panel
	panelW := float32(720)
	panelH := float32(260)
	panelX := (float32(ScreenWidth) - panelW) / 2
	panelY := (float32(ScreenHeight) - panelH) / 2

	vector.DrawFilledRect(dst, panelX, panelY, panelW, panelH, ColorBackground, false)
	vector.StrokeRect(dst, panelX, panelY, panelW, panelH, 2, ColorPrimary, false)

	cx := float64(panelX + panelW/2)
	DrawTextCentered(dst, kc.title, cx, float64(panelY)+40, FontSizeHeading, ColorText)
	DrawTextCentered(dst, "Press the new key…", cx, float64(panelY)+120, FontSizeBody, ColorPrimary)
	if kc.current != "" {
		DrawTextCentered(dst, "Currently: "+kc.current, cx, float64(panelY)+170, FontSizeSmall, ColorTextSecondary)
	}
	DrawTextCentered(dst, "Esc to cancel", cx, float64(panelY)+float64(panelH)-28, FontSizeSmall, ColorTextMuted)
}
