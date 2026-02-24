package ui

import "github.com/hajimehoshi/ebiten/v2"

// Direction represents a navigation direction.
type Direction int

const (
	DirNone Direction = iota
	DirUp
	DirDown
	DirLeft
	DirRight
)

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
	enter = ebiten.IsKeyPressed(ebiten.KeyEnter) && !prevKeys[ebiten.KeyEnter]
	back = (ebiten.IsKeyPressed(ebiten.KeyEscape) && !prevKeys[ebiten.KeyEscape]) ||
		(ebiten.IsKeyPressed(ebiten.KeyBackspace) && !prevKeys[ebiten.KeyBackspace])
	return
}

// UpdateInputState must be called at the end of each Update() to track key state.
func UpdateInputState() {
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		prevKeys[k] = ebiten.IsKeyPressed(k)
	}
	repeatTimer++
}

var (
	prevKeys   = make(map[ebiten.Key]bool)
	repeatTimer int
)

const (
	repeatDelay    = 18 // frames before repeat starts (~300ms at 60fps)
	repeatInterval = 4  // frames between repeats (~67ms at 60fps)
)

func inputRepeating(key ebiten.Key) bool {
	if !ebiten.IsKeyPressed(key) {
		return false
	}
	if !prevKeys[key] {
		return true // just pressed
	}
	// Key held — check repeat timing
	// This is a simplified version; a full impl would track per-key hold duration.
	// For now, just fire on initial press (no repeat). We can enhance later.
	return false
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
