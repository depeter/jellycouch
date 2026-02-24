package player

import "github.com/hajimehoshi/ebiten/v2"

// KeyAction represents a player action triggered by a key.
type KeyAction int

const (
	ActionNone KeyAction = iota
	ActionPlayPause
	ActionSeekForward
	ActionSeekBackward
	ActionSeekForwardLarge
	ActionSeekBackwardLarge
	ActionVolumeUp
	ActionVolumeDown
	ActionMute
	ActionSubCycle
	ActionAudioCycle
	ActionFullscreen
	ActionStop
)

var prevPlayerKeys = make(map[ebiten.Key]bool)

// PollPlayerInput checks for player-specific key presses (non-repeating).
func PollPlayerInput() KeyAction {
	defer func() {
		for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
			prevPlayerKeys[k] = ebiten.IsKeyPressed(k)
		}
	}()

	pressed := func(k ebiten.Key) bool {
		return ebiten.IsKeyPressed(k) && !prevPlayerKeys[k]
	}

	switch {
	case pressed(ebiten.KeySpace):
		return ActionPlayPause
	case pressed(ebiten.KeyRight):
		return ActionSeekForward
	case pressed(ebiten.KeyLeft):
		return ActionSeekBackward
	case pressed(ebiten.KeyUp):
		return ActionSeekForwardLarge
	case pressed(ebiten.KeyDown):
		return ActionSeekBackwardLarge
	case pressed(ebiten.KeyEqual) || pressed(ebiten.KeyKPAdd): // + key
		return ActionVolumeUp
	case pressed(ebiten.KeyMinus) || pressed(ebiten.KeyKPSubtract):
		return ActionVolumeDown
	case pressed(ebiten.KeyM):
		return ActionMute
	case pressed(ebiten.KeyS):
		return ActionSubCycle
	case pressed(ebiten.KeyA):
		return ActionAudioCycle
	case pressed(ebiten.KeyF):
		return ActionFullscreen
	case pressed(ebiten.KeyEscape), pressed(ebiten.KeyBackspace):
		return ActionStop
	}
	return ActionNone
}

// HandleAction executes a player action.
func HandleAction(p *Player, action KeyAction) {
	switch action {
	case ActionPlayPause:
		p.TogglePause()
	case ActionSeekForward:
		p.Seek(10)
	case ActionSeekBackward:
		p.Seek(-10)
	case ActionSeekForwardLarge:
		p.Seek(60)
	case ActionSeekBackwardLarge:
		p.Seek(-60)
	case ActionVolumeUp:
		// Read current, add 5
		p.mu.Lock()
		p.m.Command([]string{"add", "volume", "5"})
		p.mu.Unlock()
	case ActionVolumeDown:
		p.mu.Lock()
		p.m.Command([]string{"add", "volume", "-5"})
		p.mu.Unlock()
	case ActionMute:
		p.ToggleMute()
	case ActionSubCycle:
		p.CycleSubtitles()
	case ActionAudioCycle:
		p.CycleAudio()
	case ActionStop:
		p.Stop()
	}
}
