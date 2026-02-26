package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Direction represents a navigation direction.
type Direction int

const (
	DirNone Direction = iota
	DirUp
	DirDown
	DirLeft
	DirRight
)

// IsModifierPressed reports whether any modifier key (Alt, Ctrl, Shift, Meta) is held.
func IsModifierPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeyAlt) ||
		ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyShift) ||
		ebiten.IsKeyPressed(ebiten.KeyMeta)
}

// InputState returns the current navigation direction and action keys pressed this frame.
func InputState() (dir Direction, enter, back bool) {
	if inputRepeating(ebiten.KeyArrowUp) {
		dir = DirUp
	} else if inputRepeating(ebiten.KeyArrowDown) {
		dir = DirDown
	} else if inputRepeating(ebiten.KeyArrowLeft) {
		dir = DirLeft
	} else if inputRepeating(ebiten.KeyArrowRight) {
		dir = DirRight
	}
	enter = inpututil.IsKeyJustPressed(ebiten.KeyEnter) && !IsModifierPressed()
	back = inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeyBackspace) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) ||
		EvdevBackJustPressed()
	return
}

// UpdateInputState must be called at the end of each Update() to track key state.
func UpdateInputState() {
	// Update per-key hold frames
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if ebiten.IsKeyPressed(k) {
			keyHoldFrames[k]++
		} else {
			delete(keyHoldFrames, k)
		}
	}
}

var keyHoldFrames = make(map[ebiten.Key]int)

const (
	repeatDelay    = 18 // frames before repeat starts (~300ms at 60fps)
	repeatInterval = 4  // frames between repeats (~67ms at 60fps)
)

func inputRepeating(key ebiten.Key) bool {
	if !ebiten.IsKeyPressed(key) {
		return false
	}
	frames, held := keyHoldFrames[key]
	if !held || frames == 0 {
		return true // just pressed this frame
	}
	// Key held — check repeat timing
	if frames >= repeatDelay && (frames-repeatDelay)%repeatInterval == 0 {
		return true
	}
	return false
}

// MouseJustClicked returns the cursor position and whether the left mouse button was just clicked.
func MouseJustClicked() (x, y int, clicked bool) {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y = ebiten.CursorPosition()
		clicked = true
	}
	return
}

// MouseJustRightClicked returns the cursor position and whether the right mouse button was just clicked.
func MouseJustRightClicked() (x, y int, clicked bool) {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		x, y = ebiten.CursorPosition()
		clicked = true
	}
	return
}

// PointInRect returns true if point (px, py) is inside the rectangle (rx, ry, rw, rh).
func PointInRect(px, py int, rx, ry, rw, rh float64) bool {
	return float64(px) >= rx && float64(px) <= rx+rw &&
		float64(py) >= ry && float64(py) <= ry+rh
}

// MouseWheelDelta returns the mouse wheel scroll delta.
func MouseWheelDelta() (dx, dy float64) {
	return ebiten.Wheel()
}

// FocusGrid handles 2D grid navigation.
type FocusGrid struct {
	Cols    int
	Total   int
	Focused int
	ScrollY float64
	targetScrollY float64
}

func NewFocusGrid(cols, total int) *FocusGrid {
	return &FocusGrid{
		Cols:  cols,
		Total: total,
	}
}

func (fg *FocusGrid) Update(dir Direction) bool {
	if fg.Total == 0 {
		return false
	}
	old := fg.Focused
	row := fg.Focused / fg.Cols
	col := fg.Focused % fg.Cols

	switch dir {
	case DirLeft:
		if col > 0 {
			fg.Focused--
		}
	case DirRight:
		if col < fg.Cols-1 && fg.Focused+1 < fg.Total {
			fg.Focused++
		}
	case DirUp:
		if row > 0 {
			fg.Focused -= fg.Cols
		} else {
			return false // signal to parent: move to previous section
		}
	case DirDown:
		next := fg.Focused + fg.Cols
		if next < fg.Total {
			fg.Focused = next
		} else {
			return false // signal to parent: move to next section
		}
	}
	return fg.Focused != old
}

func (fg *FocusGrid) FocusedRow() int {
	return fg.Focused / fg.Cols
}

func (fg *FocusGrid) FocusedCol() int {
	return fg.Focused % fg.Cols
}

func (fg *FocusGrid) SetTotal(total int) {
	fg.Total = total
	if fg.Focused >= total && total > 0 {
		fg.Focused = total - 1
	}
}

// Lerp for smooth scrolling
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}
