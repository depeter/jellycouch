package ui

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var debugOverlayVisible bool

// ToggleDebugOverlay toggles the debug overlay on F12.
func ToggleDebugOverlay() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		debugOverlayVisible = !debugOverlayVisible
	}
}

// DrawDebugOverlay draws the debug overlay if visible.
func DrawDebugOverlay(screen *ebiten.Image) {
	if !debugOverlayVisible {
		return
	}

	const (
		padX    = 16.0
		padY    = 12.0
		lineH   = 18.0
		marginR = 20.0
		marginT = 20.0
	)

	// Collect data
	evdevEvents := EvdevRecentEvents()
	var pressedKeys []ebiten.Key
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if ebiten.IsKeyPressed(k) {
			pressedKeys = append(pressedKeys, k)
		}
	}

	// Calculate overlay height
	lines := 2 // header + separator
	lines += max(len(evdevEvents), 1)
	lines += 2 // blank + "Ebitengine keys:" header
	lines += max(len(pressedKeys), 1)
	panelH := float64(lines)*lineH + padY*2
	panelW := 460.0
	px := float64(ScreenWidth) - panelW - marginR
	py := marginT

	// Background
	vector.DrawFilledRect(screen, float32(px), float32(py), float32(panelW), float32(panelH), ColorOverlay, false)

	x := px + padX
	y := py + padY

	DrawText(screen, "Debug: Input Events (F12 to close)", x, y, FontSizeSmall, ColorPrimary)
	y += lineH

	DrawText(screen, "--- evdev key presses ---", x, y, FontSizeSmall, ColorTextMuted)
	y += lineH

	if len(evdevEvents) == 0 {
		DrawText(screen, "(none)", x, y, FontSizeSmall, ColorTextSecondary)
		y += lineH
	} else {
		now := time.Now()
		for _, ev := range evdevEvents {
			age := now.Sub(ev.Time).Truncate(time.Millisecond)
			line := fmt.Sprintf("%s  code=%-4d  type=%-2d  val=%d  %s ago", ev.Device, ev.Code, ev.Type, ev.Value, age)
			DrawText(screen, line, x, y, FontSizeSmall, ColorText)
			y += lineH
		}
	}

	y += lineH * 0.5
	DrawText(screen, "--- Ebitengine keys pressed ---", x, y, FontSizeSmall, ColorTextMuted)
	y += lineH

	if len(pressedKeys) == 0 {
		DrawText(screen, "(none)", x, y, FontSizeSmall, ColorTextSecondary)
	} else {
		for _, k := range pressedKeys {
			DrawText(screen, fmt.Sprintf("  %s (%d)", k.String(), int(k)), x, y, FontSizeSmall, ColorText)
			y += lineH
		}
	}
}
