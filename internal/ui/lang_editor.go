package ui

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// LangEditor is a full-screen overlay for picking and ordering languages.
type LangEditor struct {
	title      string
	allLangs   []langEntry // all language options sorted by display name
	selected   []string    // ordered codes currently chosen
	column     int         // 0=selected, 1=available
	selIndex   int         // focus index in selected column
	availIndex int         // focus index in available column
	done       bool
	result     string
}

func NewLangEditor(title string, currentCSV string) *LangEditor {
	le := &LangEditor{
		title:    title,
		allLangs: allLangEntries(),
	}
	if currentCSV != "" {
		for _, c := range strings.Split(currentCSV, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				le.selected = append(le.selected, c)
			}
		}
	}
	// Start in the available column if nothing is selected
	if len(le.selected) == 0 {
		le.column = 1
	}
	return le
}

// Done returns true when the editor should close, along with the result string.
func (le *LangEditor) Done() (bool, string) {
	return le.done, le.result
}

// available returns lang entries not in le.selected.
func (le *LangEditor) available() []langEntry {
	set := make(map[string]bool, len(le.selected))
	for _, c := range le.selected {
		set[c] = true
	}
	out := make([]langEntry, 0, len(le.allLangs))
	for _, e := range le.allLangs {
		if !set[e.Code] {
			out = append(out, e)
		}
	}
	return out
}

func (le *LangEditor) Update() {
	dir, enter, back := InputState()

	if back {
		le.result = strings.Join(le.selected, ",")
		le.done = true
		return
	}

	avail := le.available()

	// Clamp indices
	if le.selIndex >= len(le.selected) {
		le.selIndex = max(0, len(le.selected)-1)
	}
	if le.availIndex >= len(avail) {
		le.availIndex = max(0, len(avail)-1)
	}

	// Shift+Up/Down reorders in the selected column
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	if shift && le.column == 0 && len(le.selected) > 1 {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) && le.selIndex > 0 {
			le.selected[le.selIndex], le.selected[le.selIndex-1] = le.selected[le.selIndex-1], le.selected[le.selIndex]
			le.selIndex--
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && le.selIndex < len(le.selected)-1 {
			le.selected[le.selIndex], le.selected[le.selIndex+1] = le.selected[le.selIndex+1], le.selected[le.selIndex]
			le.selIndex++
			return
		}
	}

	switch dir {
	case DirUp:
		if le.column == 0 {
			if le.selIndex > 0 {
				le.selIndex--
			}
		} else {
			if le.availIndex > 0 {
				le.availIndex--
			}
		}
	case DirDown:
		if le.column == 0 {
			if le.selIndex < len(le.selected)-1 {
				le.selIndex++
			}
		} else {
			if le.availIndex < len(avail)-1 {
				le.availIndex++
			}
		}
	case DirLeft:
		if le.column == 1 && len(le.selected) > 0 {
			le.column = 0
		}
	case DirRight:
		if le.column == 0 {
			le.column = 1
		}
	}

	if enter {
		if le.column == 0 && len(le.selected) > 0 {
			// Remove from selected
			le.selected = append(le.selected[:le.selIndex], le.selected[le.selIndex+1:]...)
			if le.selIndex >= len(le.selected) {
				le.selIndex = max(0, len(le.selected)-1)
			}
			if len(le.selected) == 0 {
				le.column = 1
			}
		} else if le.column == 1 && len(avail) > 0 {
			// Add to selected
			le.selected = append(le.selected, avail[le.availIndex].Code)
			if le.availIndex >= len(avail)-1 {
				le.availIndex = max(0, len(avail)-2)
			}
		}
	}
}

func (le *LangEditor) Draw(dst *ebiten.Image) {
	// Semi-transparent overlay
	vector.DrawFilledRect(dst, 0, 0, ScreenWidth, ScreenHeight, ColorOverlay, false)

	// Centered panel
	panelW := float32(900)
	panelH := float32(600)
	panelX := float32(ScreenWidth-panelW) / 2
	panelY := float32(ScreenHeight-panelH) / 2

	vector.DrawFilledRect(dst, panelX, panelY, panelW, panelH, ColorBackground, false)
	vector.StrokeRect(dst, panelX, panelY, panelW, panelH, 2, ColorPrimary, false)

	// Title
	DrawTextCentered(dst, le.title, float64(panelX+panelW/2), float64(panelY+24), FontSizeHeading, ColorText)

	// Column layout
	colW := (panelW - 60) / 2 // 20px padding on sides, 20px gap
	leftX := panelX + 20
	rightX := panelX + 20 + colW + 20
	headerY := panelY + 56
	listY := headerY + 30

	avail := le.available()

	// Column headers
	selHeaderColor := ColorTextSecondary
	availHeaderColor := ColorTextSecondary
	if le.column == 0 {
		selHeaderColor = ColorPrimary
	} else {
		availHeaderColor = ColorPrimary
	}
	DrawText(dst, "Selected (priority order)", float64(leftX), float64(headerY), FontSizeBody, selHeaderColor)
	DrawText(dst, "Available", float64(rightX), float64(headerY), FontSizeBody, availHeaderColor)

	// Divider line
	divX := panelX + 20 + colW + 10
	vector.StrokeLine(dst, divX, headerY-4, divX, panelY+panelH-40, 1, ColorTextMuted, false)

	// Selected column
	maxRows := int((panelH - 130) / 28)
	for i, code := range le.selected {
		if i >= maxRows {
			break
		}
		iy := float32(listY) + float32(i)*28
		isFocused := le.column == 0 && i == le.selIndex
		if isFocused {
			vector.DrawFilledRect(dst, leftX, iy-2, colW, 26, ColorSurfaceHover, false)
		}
		label := fmt.Sprintf("%d. %s", i+1, langDisplayName(code))
		clr := ColorTextSecondary
		if isFocused {
			clr = ColorText
		}
		DrawText(dst, label, float64(leftX+8), float64(iy+2), FontSizeBody, clr)
	}
	if len(le.selected) == 0 {
		DrawText(dst, "No languages selected", float64(leftX+8), float64(listY+2), FontSizeSmall, ColorTextMuted)
	}

	// Available column
	for i, entry := range avail {
		if i >= maxRows {
			break
		}
		iy := float32(listY) + float32(i)*28
		isFocused := le.column == 1 && i == le.availIndex
		if isFocused {
			vector.DrawFilledRect(dst, rightX, iy-2, colW, 26, ColorSurfaceHover, false)
		}
		clr := ColorTextSecondary
		if isFocused {
			clr = ColorText
		}
		DrawText(dst, entry.Name, float64(rightX+8), float64(iy+2), FontSizeBody, clr)
	}

	// Hint bar at bottom
	hint := "Enter: Toggle  |  Shift+\u2191/\u2193: Reorder  |  \u2190/\u2192: Switch Column  |  Esc: Done"
	DrawTextCentered(dst, hint, float64(panelX+panelW/2), float64(panelY+panelH-16), FontSizeSmall, ColorTextMuted)
}
