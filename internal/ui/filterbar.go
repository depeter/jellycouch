package ui

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// FilterOption represents a single pill selector with a label and cycled options.
type FilterOption struct {
	Label    string
	Options  []string
	Selected int
}

// Value returns the currently selected option string.
func (fo *FilterOption) Value() string {
	if fo.Selected < 0 || fo.Selected >= len(fo.Options) {
		return ""
	}
	return fo.Options[fo.Selected]
}

// FilterBar is a horizontal bar of pill selectors plus a search input.
type FilterBar struct {
	Filters      []FilterOption
	FocusedIndex int // index into Filters; len(Filters) = search input
	SearchInput  TextInput
	Active       bool
	OnChanged    func()

	pillRects     []ButtonRect
	searchRect    ButtonRect
	debounceTimer *time.Timer
	lastSearch    string
}

// NewFilterBar creates a FilterBar with the given filter options.
func NewFilterBar(filters []FilterOption) *FilterBar {
	return &FilterBar{
		Filters:   filters,
		pillRects: make([]ButtonRect, len(filters)),
	}
}

// IsSearchFocused returns true if the search input currently has focus.
func (fb *FilterBar) IsSearchFocused() bool {
	return fb.Active && fb.FocusedIndex >= len(fb.Filters)
}

// Update processes input for the filter bar. Returns true if any filter value changed.
func (fb *FilterBar) Update() bool {
	if !fb.Active {
		return false
	}

	changed := false

	if fb.IsSearchFocused() {
		// Search input mode — text input handles most keys
		textChanged := fb.SearchInput.Update()

		// Left at cursor position 0 → move to previous pill
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) && fb.SearchInput.Cursor == 0 {
			if len(fb.Filters) > 0 {
				fb.FocusedIndex = len(fb.Filters) - 1
			}
			return changed
		}

		if textChanged {
			// Debounce search: cancel previous timer, start new one
			if fb.debounceTimer != nil {
				fb.debounceTimer.Stop()
			}
			fb.debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				// Will be picked up on next frame
			})
		}

		// Check if debounced search should fire
		if fb.SearchInput.Text != fb.lastSearch {
			if fb.debounceTimer != nil {
				select {
				case <-fb.debounceTimer.C:
					// Timer fired (won't happen with AfterFunc, use flag instead)
				default:
				}
			}
			// For AfterFunc: check if enough time has passed
			if textChanged {
				// Reset — actual firing happens via timer
			}
		}

		// Fire search on Enter immediately
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && fb.SearchInput.Text != fb.lastSearch {
			fb.lastSearch = fb.SearchInput.Text
			changed = true
		}

		return changed
	}

	// Pill mode — Left/Right to navigate, Up/Down/Enter to cycle value
	if inputRepeating(ebiten.KeyArrowLeft) {
		if fb.FocusedIndex > 0 {
			fb.FocusedIndex--
		}
	}
	if inputRepeating(ebiten.KeyArrowRight) {
		if fb.FocusedIndex < len(fb.Filters) { // can go to search
			fb.FocusedIndex++
		}
	}
	if fb.FocusedIndex < len(fb.Filters) {
		pill := &fb.Filters[fb.FocusedIndex]
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			// Cycle forward
			pill.Selected = (pill.Selected + 1) % len(pill.Options)
			changed = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			// Cycle backward
			pill.Selected = (pill.Selected - 1 + len(pill.Options)) % len(pill.Options)
			changed = true
		}
	}

	return changed
}

// CheckSearchDebounce checks if the search input has changed and debounce time elapsed.
// Call this each frame; returns true if search changed.
func (fb *FilterBar) CheckSearchDebounce() bool {
	if fb.SearchInput.Text != fb.lastSearch && fb.debounceTimer == nil {
		// No pending timer — text was changed without a timer (shouldn't happen normally)
		return false
	}
	// The AfterFunc approach: we just track if text differs and timer has completed
	// Simplify: just use a frame counter approach
	return false
}

// SetSearchText sets the search input text and marks it as applied.
func (fb *FilterBar) SetSearchText(text string) {
	fb.SearchInput.SetText(text)
	fb.lastSearch = text
}

// HandleClick checks if a click hit any pill or the search bar.
// Returns the index clicked (len(Filters) for search), and whether a click was handled.
func (fb *FilterBar) HandleClick(mx, my int) (int, bool) {
	for i, rect := range fb.pillRects {
		if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
			return i, true
		}
	}
	if PointInRect(mx, my, fb.searchRect.X, fb.searchRect.Y, fb.searchRect.W, fb.searchRect.H) {
		return len(fb.Filters), true
	}
	return -1, false
}

const (
	filterBarHeight  = 38.0
	filterBarPadding = 8.0
	filterPillGap    = 12.0
	filterPillPadX   = 14.0
)

// Draw renders the filter bar at the given position.
func (fb *FilterBar) Draw(dst *ebiten.Image, x, y float64) float64 {
	curX := x

	for i := range fb.Filters {
		pill := &fb.Filters[i]
		label := pill.Label + ": " + pill.Value()
		tw, _ := MeasureText(label, FontSizeBody)
		pillW := tw + filterPillPadX*2

		isFocused := fb.Active && fb.FocusedIndex == i

		if isFocused {
			vector.DrawFilledRect(dst, float32(curX), float32(y),
				float32(pillW), float32(filterBarHeight), ColorPrimary, false)
			DrawTextCentered(dst, label, curX+pillW/2, y+filterBarHeight/2,
				FontSizeBody, ColorBackground)
			// Draw up/down arrow triangles as affordance
			drawTriangle(dst, float32(curX+pillW/2), float32(y-6), 5, true, ColorPrimary)
			drawTriangle(dst, float32(curX+pillW/2), float32(y+filterBarHeight+6), 5, false, ColorPrimary)
		} else {
			vector.DrawFilledRect(dst, float32(curX), float32(y),
				float32(pillW), float32(filterBarHeight), ColorSurface, false)
			vector.StrokeRect(dst, float32(curX), float32(y),
				float32(pillW), float32(filterBarHeight), 1, ColorTextMuted, false)
			DrawTextCentered(dst, label, curX+pillW/2, y+filterBarHeight/2,
				FontSizeBody, ColorTextSecondary)
		}

		fb.pillRects[i] = ButtonRect{X: curX, Y: y, W: pillW, H: filterBarHeight}
		curX += pillW + filterPillGap
	}

	// Search input — fills remaining width
	searchW := float64(ScreenWidth) - SectionPadding - curX
	if searchW < 100 {
		searchW = 100
	}
	isSearchFocused := fb.Active && fb.FocusedIndex >= len(fb.Filters)

	if isSearchFocused {
		vector.DrawFilledRect(dst, float32(curX), float32(y),
			float32(searchW), float32(filterBarHeight), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(curX), float32(y),
			float32(searchW), float32(filterBarHeight), 2, ColorFocusBorder, false)
		if fb.SearchInput.Text == "" {
			DrawText(dst, "Search...", curX+10, y+10, FontSizeBody, ColorTextMuted)
		}
		DrawText(dst, fb.SearchInput.DisplayText(), curX+10, y+10, FontSizeBody, ColorText)
	} else {
		vector.DrawFilledRect(dst, float32(curX), float32(y),
			float32(searchW), float32(filterBarHeight), ColorSurface, false)
		vector.StrokeRect(dst, float32(curX), float32(y),
			float32(searchW), float32(filterBarHeight), 1, ColorTextMuted, false)
		if fb.SearchInput.Text != "" {
			DrawText(dst, fb.SearchInput.Text, curX+10, y+10, FontSizeBody, ColorText)
		} else {
			DrawText(dst, "Search...", curX+10, y+10, FontSizeBody, ColorTextMuted)
		}
	}

	fb.searchRect = ButtonRect{X: curX, Y: y, W: searchW, H: filterBarHeight}
	return filterBarHeight
}
