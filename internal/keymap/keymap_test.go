package keymap

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestParseKnownKeys(t *testing.T) {
	cases := []struct {
		in   string
		want ebiten.Key
	}{
		{"Space", ebiten.KeySpace},
		{"space", ebiten.KeySpace},
		{"Left", ebiten.KeyArrowLeft},
		{"ArrowLeft", ebiten.KeyArrowLeft},
		{"Digit0", ebiten.KeyDigit0},
		{"0", ebiten.KeyDigit0},
		{"M", ebiten.KeyM},
		{"m", ebiten.KeyM},
		{"Esc", ebiten.KeyEscape},
		{"Escape", ebiten.KeyEscape},
	}
	for _, c := range cases {
		got, ok := parse(c.in)
		if !ok {
			t.Errorf("parse(%q) failed", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("parse(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseUnknownKey(t *testing.T) {
	if _, ok := parse("NotAKey"); ok {
		t.Error("parse should fail for unknown key")
	}
}

func TestDefaultResolve(t *testing.T) {
	kb := Resolve(Default())
	if kb.PlayPause != ebiten.KeySpace {
		t.Errorf("default play_pause = %v, want Space", kb.PlayPause)
	}
	if kb.Mute != ebiten.KeyM {
		t.Errorf("default mute = %v, want M", kb.Mute)
	}
	if kb.VolumeUp != ebiten.KeyDigit0 {
		t.Errorf("default volume_up = %v, want Digit0", kb.VolumeUp)
	}
}

func TestResolveUnboundEmpty(t *testing.T) {
	c := Config{}
	kb := Resolve(c)
	if IsBound(kb.PlayPause) {
		t.Errorf("empty play_pause should be unbound, got %v", kb.PlayPause)
	}
}

// TestNameParseRoundTrip ensures every key Name() produces is also
// round-trippable through parse(). Keeps the two functions from drifting.
func TestNameParseRoundTrip(t *testing.T) {
	keys := []ebiten.Key{
		ebiten.KeySpace, ebiten.KeyArrowRight, ebiten.KeyArrowLeft,
		ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyEnter,
		ebiten.KeyEscape, ebiten.KeyBackspace,
		ebiten.KeyDigit0, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3,
		ebiten.KeyDigit4, ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7,
		ebiten.KeyDigit8, ebiten.KeyDigit9,
		ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE,
		ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO,
		ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY,
		ebiten.KeyZ,
	}
	for _, k := range keys {
		name := Name(k)
		if name == "" {
			t.Errorf("Name(%v) returned empty", k)
			continue
		}
		got, ok := parse(name)
		if !ok || got != k {
			t.Errorf("round trip failed: Name(%v)=%q parse=%v ok=%v", k, name, got, ok)
		}
	}
}

func TestNameUnsupported(t *testing.T) {
	if Name(ebiten.KeyF1) != "" {
		t.Error("F1 should not have a config name (not remappable)")
	}
}
