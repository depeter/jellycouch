package ui

import (
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// TextInput handles text editing with cursor navigation.
type TextInput struct {
	Text   string
	Cursor int // rune position within Text
}

// NewTextInput creates a TextInput initialized with the given text and cursor at the end.
func NewTextInput(text string) TextInput {
	return TextInput{
		Text:   text,
		Cursor: utf8.RuneCountInString(text),
	}
}

// SetText replaces the text and moves cursor to the end.
func (ti *TextInput) SetText(text string) {
	ti.Text = text
	ti.Cursor = utf8.RuneCountInString(text)
}

// Clear resets the text and cursor.
func (ti *TextInput) Clear() {
	ti.Text = ""
	ti.Cursor = 0
}

// Update processes input events. Returns true if the text changed.
func (ti *TextInput) Update() bool {
	changed := false
	runeCount := utf8.RuneCountInString(ti.Text)

	// Cursor movement
	if inputRepeating(ebiten.KeyArrowLeft) {
		if ti.Cursor > 0 {
			ti.Cursor--
		}
	}
	if inputRepeating(ebiten.KeyArrowRight) {
		if ti.Cursor < runeCount {
			ti.Cursor++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		ti.Cursor = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		ti.Cursor = runeCount
	}

	// Ctrl+V paste from clipboard
	if inpututil.IsKeyJustPressed(ebiten.KeyV) && (ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)) {
		if clip := readClipboard(); clip != "" {
			ti.insertAtCursor(clip)
			changed = true
		}
	}

	// Character input
	runes := ebiten.AppendInputChars(nil)
	for _, r := range runes {
		if !unicode.IsControl(r) {
			ti.insertAtCursor(string(r))
			changed = true
		}
	}

	// Backspace — delete before cursor
	if inputRepeating(ebiten.KeyBackspace) && ti.Cursor > 0 {
		before, after := ti.splitAtCursor()
		_, size := utf8.DecodeLastRuneInString(before)
		ti.Text = before[:len(before)-size] + after
		ti.Cursor--
		changed = true
	}

	// Delete — delete after cursor
	if inputRepeating(ebiten.KeyDelete) && ti.Cursor < runeCount {
		_, after := ti.splitAtCursor()
		before := ti.Text[:len(ti.Text)-len(after)]
		_, size := utf8.DecodeRuneInString(after)
		ti.Text = before + after[size:]
		changed = true
	}

	return changed
}

// DisplayText returns the text with a cursor indicator inserted at the cursor position.
func (ti *TextInput) DisplayText() string {
	before, after := ti.splitAtCursor()
	return before + "│" + after
}

func (ti *TextInput) insertAtCursor(s string) {
	before, after := ti.splitAtCursor()
	ti.Text = before + s + after
	ti.Cursor += utf8.RuneCountInString(s)
}

// splitAtCursor returns the text before and after the cursor position.
func (ti *TextInput) splitAtCursor() (before, after string) {
	bytePos := 0
	for i := 0; i < ti.Cursor; i++ {
		_, size := utf8.DecodeRuneInString(ti.Text[bytePos:])
		bytePos += size
	}
	return ti.Text[:bytePos], ti.Text[bytePos:]
}

// CursorAtEnd reports whether the cursor is at the end of the text.
func (ti *TextInput) CursorAtEnd() bool {
	return ti.Cursor >= utf8.RuneCountInString(ti.Text)
}

// readClipboard is implemented per-platform in clipboard_*.go
