// Package keymap parses config-specified key names into ebiten.Key values.
// Used to let users remap playback controls via config.toml without
// recompiling.
package keymap

import (
	"log/slog"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// Unbound is the sentinel value for a disabled binding. Guards against the
// Go zero value colliding with ebiten.KeyA (which is Key(0)).
const Unbound ebiten.Key = -1

// Keybindings holds all remappable actions. Use Unbound (not zero) for
// disabled bindings; callers should check via IsBound.
type Keybindings struct {
	PlayPause        ebiten.Key
	SeekSmallForward ebiten.Key
	SeekSmallBack    ebiten.Key
	SeekLargeForward ebiten.Key
	SeekLargeBack    ebiten.Key
	VolumeUp         ebiten.Key
	VolumeDown       ebiten.Key
	Mute             ebiten.Key
	CycleSubtitles   ebiten.Key
	CycleAudio       ebiten.Key
	ShowInfo         ebiten.Key
}

// Config is the TOML-serializable form of Keybindings. Each field is a
// human-readable key name (e.g. "Space", "Left", "Digit0", "M"). Empty
// string means unbound.
type Config struct {
	PlayPause        string `toml:"play_pause"`
	SeekSmallForward string `toml:"seek_small_forward"`
	SeekSmallBack    string `toml:"seek_small_back"`
	SeekLargeForward string `toml:"seek_large_forward"`
	SeekLargeBack    string `toml:"seek_large_back"`
	VolumeUp         string `toml:"volume_up"`
	VolumeDown       string `toml:"volume_down"`
	Mute             string `toml:"mute"`
	CycleSubtitles   string `toml:"cycle_subtitles"`
	CycleAudio       string `toml:"cycle_audio"`
	ShowInfo         string `toml:"show_info"`
}

// Default returns the built-in keybinding set matching the original
// hardcoded bindings.
func Default() Config {
	return Config{
		PlayPause:        "Space",
		SeekSmallForward: "ArrowRight",
		SeekSmallBack:    "ArrowLeft",
		SeekLargeForward: "ArrowUp",
		SeekLargeBack:    "ArrowDown",
		VolumeUp:         "Digit0",
		VolumeDown:       "Digit9",
		Mute:             "M",
		CycleSubtitles:   "S",
		CycleAudio:       "A",
		ShowInfo:         "I",
	}
}

// Resolve parses a Config into Keybindings. Unknown names are logged and
// left unbound (zero value).
func Resolve(c Config) Keybindings {
	pick := func(field, name string) ebiten.Key {
		if name == "" {
			return Unbound
		}
		if k, ok := parse(name); ok {
			return k
		}
		slog.Warn("unknown keybinding, leaving unbound", "field", field, "name", name)
		return Unbound
	}
	return Keybindings{
		PlayPause:        pick("play_pause", c.PlayPause),
		SeekSmallForward: pick("seek_small_forward", c.SeekSmallForward),
		SeekSmallBack:    pick("seek_small_back", c.SeekSmallBack),
		SeekLargeForward: pick("seek_large_forward", c.SeekLargeForward),
		SeekLargeBack:    pick("seek_large_back", c.SeekLargeBack),
		VolumeUp:         pick("volume_up", c.VolumeUp),
		VolumeDown:       pick("volume_down", c.VolumeDown),
		Mute:             pick("mute", c.Mute),
		CycleSubtitles:   pick("cycle_subtitles", c.CycleSubtitles),
		CycleAudio:       pick("cycle_audio", c.CycleAudio),
		ShowInfo:         pick("show_info", c.ShowInfo),
	}
}

// IsBound returns true if k is a real ebiten key (not the Unbound sentinel).
func IsBound(k ebiten.Key) bool {
	return k != Unbound
}

// Name returns the canonical config name for an ebiten key. Used when the
// UI captures a live keypress and writes it back to config. Returns ""
// for keys we don't support remapping (callers should treat this as
// "please pick a different key").
func Name(k ebiten.Key) string {
	switch k {
	case ebiten.KeySpace:
		return "Space"
	case ebiten.KeyArrowRight:
		return "ArrowRight"
	case ebiten.KeyArrowLeft:
		return "ArrowLeft"
	case ebiten.KeyArrowUp:
		return "ArrowUp"
	case ebiten.KeyArrowDown:
		return "ArrowDown"
	case ebiten.KeyEnter:
		return "Enter"
	case ebiten.KeyEscape:
		return "Escape"
	case ebiten.KeyBackspace:
		return "Backspace"
	case ebiten.KeyDigit0:
		return "Digit0"
	case ebiten.KeyDigit1:
		return "Digit1"
	case ebiten.KeyDigit2:
		return "Digit2"
	case ebiten.KeyDigit3:
		return "Digit3"
	case ebiten.KeyDigit4:
		return "Digit4"
	case ebiten.KeyDigit5:
		return "Digit5"
	case ebiten.KeyDigit6:
		return "Digit6"
	case ebiten.KeyDigit7:
		return "Digit7"
	case ebiten.KeyDigit8:
		return "Digit8"
	case ebiten.KeyDigit9:
		return "Digit9"
	case ebiten.KeyA:
		return "A"
	case ebiten.KeyB:
		return "B"
	case ebiten.KeyC:
		return "C"
	case ebiten.KeyD:
		return "D"
	case ebiten.KeyE:
		return "E"
	case ebiten.KeyF:
		return "F"
	case ebiten.KeyG:
		return "G"
	case ebiten.KeyH:
		return "H"
	case ebiten.KeyI:
		return "I"
	case ebiten.KeyJ:
		return "J"
	case ebiten.KeyK:
		return "K"
	case ebiten.KeyL:
		return "L"
	case ebiten.KeyM:
		return "M"
	case ebiten.KeyN:
		return "N"
	case ebiten.KeyO:
		return "O"
	case ebiten.KeyP:
		return "P"
	case ebiten.KeyQ:
		return "Q"
	case ebiten.KeyR:
		return "R"
	case ebiten.KeyS:
		return "S"
	case ebiten.KeyT:
		return "T"
	case ebiten.KeyU:
		return "U"
	case ebiten.KeyV:
		return "V"
	case ebiten.KeyW:
		return "W"
	case ebiten.KeyX:
		return "X"
	case ebiten.KeyY:
		return "Y"
	case ebiten.KeyZ:
		return "Z"
	}
	return ""
}

// parse converts a key name to an ebiten.Key. Case-insensitive. Covers the
// subset of keys currently used for playback control.
func parse(name string) (ebiten.Key, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "space":
		return ebiten.KeySpace, true
	case "arrowright", "right":
		return ebiten.KeyArrowRight, true
	case "arrowleft", "left":
		return ebiten.KeyArrowLeft, true
	case "arrowup", "up":
		return ebiten.KeyArrowUp, true
	case "arrowdown", "down":
		return ebiten.KeyArrowDown, true
	case "enter", "return":
		return ebiten.KeyEnter, true
	case "escape", "esc":
		return ebiten.KeyEscape, true
	case "backspace":
		return ebiten.KeyBackspace, true
	case "digit0", "0":
		return ebiten.KeyDigit0, true
	case "digit9", "9":
		return ebiten.KeyDigit9, true
	case "digit1", "1":
		return ebiten.KeyDigit1, true
	case "digit2", "2":
		return ebiten.KeyDigit2, true
	case "digit3", "3":
		return ebiten.KeyDigit3, true
	case "digit4", "4":
		return ebiten.KeyDigit4, true
	case "digit5", "5":
		return ebiten.KeyDigit5, true
	case "digit6", "6":
		return ebiten.KeyDigit6, true
	case "digit7", "7":
		return ebiten.KeyDigit7, true
	case "digit8", "8":
		return ebiten.KeyDigit8, true
	case "a":
		return ebiten.KeyA, true
	case "b":
		return ebiten.KeyB, true
	case "c":
		return ebiten.KeyC, true
	case "d":
		return ebiten.KeyD, true
	case "e":
		return ebiten.KeyE, true
	case "f":
		return ebiten.KeyF, true
	case "g":
		return ebiten.KeyG, true
	case "h":
		return ebiten.KeyH, true
	case "i":
		return ebiten.KeyI, true
	case "j":
		return ebiten.KeyJ, true
	case "k":
		return ebiten.KeyK, true
	case "l":
		return ebiten.KeyL, true
	case "m":
		return ebiten.KeyM, true
	case "n":
		return ebiten.KeyN, true
	case "o":
		return ebiten.KeyO, true
	case "p":
		return ebiten.KeyP, true
	case "q":
		return ebiten.KeyQ, true
	case "r":
		return ebiten.KeyR, true
	case "s":
		return ebiten.KeyS, true
	case "t":
		return ebiten.KeyT, true
	case "u":
		return ebiten.KeyU, true
	case "v":
		return ebiten.KeyV, true
	case "w":
		return ebiten.KeyW, true
	case "x":
		return ebiten.KeyX, true
	case "y":
		return ebiten.KeyY, true
	case "z":
		return ebiten.KeyZ, true
	}
	return 0, false
}
