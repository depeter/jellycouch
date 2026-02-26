package app

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// keyMap maps config key names to ebiten keys.
var keyMap = map[string]ebiten.Key{
	"space":  ebiten.KeySpace,
	"enter":  ebiten.KeyEnter,
	"return": ebiten.KeyEnter,
	"tab":    ebiten.KeyTab,
	"left":   ebiten.KeyArrowLeft,
	"right":  ebiten.KeyArrowRight,
	"up":     ebiten.KeyArrowUp,
	"down":   ebiten.KeyArrowDown,
	"a":      ebiten.KeyA,
	"b":      ebiten.KeyB,
	"c":      ebiten.KeyC,
	"d":      ebiten.KeyD,
	"e":      ebiten.KeyE,
	"f":      ebiten.KeyF,
	"g":      ebiten.KeyG,
	"h":      ebiten.KeyH,
	"i":      ebiten.KeyI,
	"j":      ebiten.KeyJ,
	"k":      ebiten.KeyK,
	"l":      ebiten.KeyL,
	"m":      ebiten.KeyM,
	"n":      ebiten.KeyN,
	"o":      ebiten.KeyO,
	"p":      ebiten.KeyP,
	"q":      ebiten.KeyQ,
	"r":      ebiten.KeyR,
	"s":      ebiten.KeyS,
	"t":      ebiten.KeyT,
	"u":      ebiten.KeyU,
	"v":      ebiten.KeyV,
	"w":      ebiten.KeyW,
	"x":      ebiten.KeyX,
	"y":      ebiten.KeyY,
	"z":      ebiten.KeyZ,
	"0":      ebiten.KeyDigit0,
	"1":      ebiten.KeyDigit1,
	"2":      ebiten.KeyDigit2,
	"3":      ebiten.KeyDigit3,
	"4":      ebiten.KeyDigit4,
	"5":      ebiten.KeyDigit5,
	"6":      ebiten.KeyDigit6,
	"7":      ebiten.KeyDigit7,
	"8":      ebiten.KeyDigit8,
	"9":      ebiten.KeyDigit9,
}

// parseKey converts a config key name to an ebiten.Key.
func parseKey(name string) (ebiten.Key, bool) {
	k, ok := keyMap[strings.ToLower(name)]
	return k, ok
}

// keyJustPressed checks if the key named by the config string was just pressed.
func keyJustPressed(name string) bool {
	if k, ok := parseKey(name); ok {
		return inpututil.IsKeyJustPressed(k)
	}
	return false
}
