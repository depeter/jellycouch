package ui

import (
	"fmt"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/config"
)

// SettingsScreen allows editing configuration.
type SettingsScreen struct {
	cfg *config.Config

	sections []settingsSection
	sectionIndex int
	itemIndex    int
	editing      bool
	editInput    TextInput
	editError    string

	// Row rects for mouse clicks (flat list across all sections)
	rowRects []settingsRowRect

	OnSave func()
}

type settingsRowRect struct {
	SectionIdx int
	ItemIdx    int
	X, Y, W, H float64
}

type settingsSection struct {
	Label string
	Items []settingsItem
}

type settingsItem struct {
	Label    string
	Value    func() string
	OnChange func(val string) error // returns error if validation fails
}

func NewSettingsScreen(cfg *config.Config, onSave func()) *SettingsScreen {
	ss := &SettingsScreen{
		cfg:    cfg,
		OnSave: onSave,
	}

	ss.sections = []settingsSection{
		{
			Label: "Server",
			Items: []settingsItem{
				{Label: "Server URL", Value: func() string { return cfg.Server.URL }, OnChange: func(v string) error { cfg.Server.URL = v; return nil }},
				{Label: "Username", Value: func() string { return cfg.Server.Username }, OnChange: func(v string) error { cfg.Server.Username = v; return nil }},
			},
		},
		{
			Label: "Subtitles",
			Items: []settingsItem{
				{Label: "Font", Value: func() string { return cfg.Subtitles.Font }, OnChange: func(v string) error { cfg.Subtitles.Font = v; return nil }},
				{Label: "Font Size", Value: func() string { return fmt.Sprintf("%d", cfg.Subtitles.FontSize) }, OnChange: func(v string) error {
					n, err := strconv.Atoi(v)
					if err != nil {
						return fmt.Errorf("invalid number: %s", v)
					}
					cfg.Subtitles.FontSize = n
					return nil
				}},
				{Label: "Color", Value: func() string { return cfg.Subtitles.Color }, OnChange: func(v string) error { cfg.Subtitles.Color = v; return nil }},
				{Label: "Border Size", Value: func() string { return fmt.Sprintf("%.1f", cfg.Subtitles.BorderSize) }, OnChange: func(v string) error {
					f, err := strconv.ParseFloat(v, 64)
					if err != nil {
						return fmt.Errorf("invalid number: %s", v)
					}
					cfg.Subtitles.BorderSize = f
					return nil
				}},
				{Label: "Position", Value: func() string { return fmt.Sprintf("%d", cfg.Subtitles.Position) }, OnChange: func(v string) error {
					n, err := strconv.Atoi(v)
					if err != nil {
						return fmt.Errorf("invalid number: %s", v)
					}
					cfg.Subtitles.Position = n
					return nil
				}},
			},
		},
		{
			Label: "Playback",
			Items: []settingsItem{
				{Label: "HW Accel", Value: func() string { return cfg.Playback.HWAccel }, OnChange: func(v string) error { cfg.Playback.HWAccel = v; return nil }},
				{Label: "Audio Language", Value: func() string { return cfg.Playback.AudioLanguage }, OnChange: func(v string) error { cfg.Playback.AudioLanguage = v; return nil }},
				{Label: "Sub Language", Value: func() string { return cfg.Playback.SubLanguage }, OnChange: func(v string) error { cfg.Playback.SubLanguage = v; return nil }},
				{Label: "Volume", Value: func() string { return fmt.Sprintf("%d", cfg.Playback.Volume) }, OnChange: func(v string) error {
					n, err := strconv.Atoi(v)
					if err != nil {
						return fmt.Errorf("invalid number: %s", v)
					}
					cfg.Playback.Volume = n
					return nil
				}},
			},
		},
	}

	return ss
}

func (ss *SettingsScreen) Name() string { return "Settings" }
func (ss *SettingsScreen) OnEnter()     {}
func (ss *SettingsScreen) OnExit() {
	if ss.OnSave != nil {
		ss.OnSave()
	}
}

func (ss *SettingsScreen) Update() (*ScreenTransition, error) {
	_, enter, back := InputState()

	if ss.editing {
		if ss.editInput.Update() {
			ss.editError = "" // clear error as user types
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			// Apply edit with validation
			sec := ss.sections[ss.sectionIndex]
			if err := sec.Items[ss.itemIndex].OnChange(ss.editInput.Text); err != nil {
				ss.editError = err.Error()
			} else {
				ss.editing = false
				ss.editError = ""
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			ss.editing = false
			ss.editError = ""
		}
		return nil, nil
	}

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked {
		for _, rect := range ss.rowRects {
			if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
				ss.sectionIndex = rect.SectionIdx
				ss.itemIndex = rect.ItemIdx
				// Enter edit mode
				sec := ss.sections[ss.sectionIndex]
				ss.editInput = NewTextInput(sec.Items[ss.itemIndex].Value())
				ss.editing = true
				ss.editError = ""
				return nil, nil
			}
		}
	}

	dir, _, _ := InputState()
	switch dir {
	case DirUp:
		ss.itemIndex--
		if ss.itemIndex < 0 {
			ss.sectionIndex--
			if ss.sectionIndex < 0 {
				ss.sectionIndex = 0
				ss.itemIndex = 0
			} else {
				ss.itemIndex = len(ss.sections[ss.sectionIndex].Items) - 1
			}
		}
	case DirDown:
		ss.itemIndex++
		if ss.itemIndex >= len(ss.sections[ss.sectionIndex].Items) {
			ss.sectionIndex++
			if ss.sectionIndex >= len(ss.sections) {
				ss.sectionIndex = len(ss.sections) - 1
				ss.itemIndex = len(ss.sections[ss.sectionIndex].Items) - 1
			} else {
				ss.itemIndex = 0
			}
		}
	}

	if enter {
		sec := ss.sections[ss.sectionIndex]
		ss.editInput = NewTextInput(sec.Items[ss.itemIndex].Value())
		ss.editing = true
		ss.editError = ""
	}

	return nil, nil
}

func (ss *SettingsScreen) Draw(dst *ebiten.Image) {
	DrawText(dst, "Settings", SectionPadding, 16, FontSizeTitle, ColorText)

	y := float64(NavBarHeight + 10)
	ss.rowRects = ss.rowRects[:0] // reset

	for si, sec := range ss.sections {
		DrawText(dst, sec.Label, SectionPadding, y, FontSizeHeading, ColorPrimary)
		y += FontSizeHeading + 8

		for ii, item := range sec.Items {
			isFocused := si == ss.sectionIndex && ii == ss.itemIndex
			rowH := float32(40)
			rowX := float64(SectionPadding - 8)
			rowW := float64(ScreenWidth - SectionPadding*2 + 16)

			// Store rect for mouse clicks
			ss.rowRects = append(ss.rowRects, settingsRowRect{
				SectionIdx: si, ItemIdx: ii,
				X: rowX, Y: y - 4, W: rowW, H: float64(rowH),
			})

			if isFocused {
				vector.DrawFilledRect(dst, float32(rowX), float32(y-4),
					float32(rowW), rowH, ColorSurfaceHover, false)
			}

			labelColor := ColorTextSecondary
			if isFocused {
				labelColor = ColorText
			}
			DrawText(dst, item.Label, SectionPadding, y+4, FontSizeBody, labelColor)

			value := item.Value()
			isEditing := ss.editing && isFocused
			if isEditing {
				value = ss.editInput.DisplayText()
				// Blue border around value field when editing
				valueX := float32(SectionPadding + 296)
				valueW := float32(rowW) - float32(300) - 8
				vector.StrokeRect(dst, valueX, float32(y-2), valueW, float32(rowH)-4, 2, ColorFocusBorder, false)
			}
			DrawText(dst, value, SectionPadding+300, y+4, FontSizeBody, ColorTextSecondary)

			// Show edit error below the row
			if isEditing && ss.editError != "" {
				DrawText(dst, ss.editError, SectionPadding+300, y+float64(rowH)-4, FontSizeSmall, ColorError)
			}

			y += float64(rowH)
		}
		y += 16
	}

	// Bottom hint
	hint := "Esc to go back (auto-saves)"
	if ss.editing {
		hint = "Enter to save, Esc to cancel"
	}
	DrawText(dst, hint, SectionPadding, float64(ScreenHeight)-40,
		FontSizeSmall, ColorTextMuted)
}
