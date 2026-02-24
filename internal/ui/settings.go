package ui

import (
	"fmt"

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
	editValue    string

	OnSave func()
}

type settingsSection struct {
	Label string
	Items []settingsItem
}

type settingsItem struct {
	Label    string
	Value    func() string
	OnChange func(val string)
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
				{Label: "Server URL", Value: func() string { return cfg.Server.URL }, OnChange: func(v string) { cfg.Server.URL = v }},
				{Label: "Username", Value: func() string { return cfg.Server.Username }, OnChange: func(v string) { cfg.Server.Username = v }},
			},
		},
		{
			Label: "Subtitles",
			Items: []settingsItem{
				{Label: "Font", Value: func() string { return cfg.Subtitles.Font }, OnChange: func(v string) { cfg.Subtitles.Font = v }},
				{Label: "Font Size", Value: func() string { return fmt.Sprintf("%d", cfg.Subtitles.FontSize) }, OnChange: func(v string) { fmt.Sscanf(v, "%d", &cfg.Subtitles.FontSize) }},
				{Label: "Color", Value: func() string { return cfg.Subtitles.Color }, OnChange: func(v string) { cfg.Subtitles.Color = v }},
				{Label: "Border Size", Value: func() string { return fmt.Sprintf("%.1f", cfg.Subtitles.BorderSize) }, OnChange: func(v string) { fmt.Sscanf(v, "%f", &cfg.Subtitles.BorderSize) }},
				{Label: "Position", Value: func() string { return fmt.Sprintf("%d", cfg.Subtitles.Position) }, OnChange: func(v string) { fmt.Sscanf(v, "%d", &cfg.Subtitles.Position) }},
			},
		},
		{
			Label: "Playback",
			Items: []settingsItem{
				{Label: "HW Accel", Value: func() string { return cfg.Playback.HWAccel }, OnChange: func(v string) { cfg.Playback.HWAccel = v }},
				{Label: "Audio Language", Value: func() string { return cfg.Playback.AudioLanguage }, OnChange: func(v string) { cfg.Playback.AudioLanguage = v }},
				{Label: "Sub Language", Value: func() string { return cfg.Playback.SubLanguage }, OnChange: func(v string) { cfg.Playback.SubLanguage = v }},
				{Label: "Volume", Value: func() string { return fmt.Sprintf("%d", cfg.Playback.Volume) }, OnChange: func(v string) { fmt.Sscanf(v, "%d", &cfg.Playback.Volume) }},
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
		runes := ebiten.AppendInputChars(nil)
		if len(runes) > 0 {
			ss.editValue += string(runes)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(ss.editValue) > 0 {
			ss.editValue = ss.editValue[:len(ss.editValue)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			// Apply edit
			sec := ss.sections[ss.sectionIndex]
			sec.Items[ss.itemIndex].OnChange(ss.editValue)
			ss.editing = false
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			ss.editing = false
		}
		return nil, nil
	}

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
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
		ss.editValue = sec.Items[ss.itemIndex].Value()
		ss.editing = true
	}

	return nil, nil
}

func (ss *SettingsScreen) Draw(dst *ebiten.Image) {
	DrawText(dst, "Settings", SectionPadding, 16, FontSizeTitle, ColorText)

	y := float64(NavBarHeight + 10)

	for si, sec := range ss.sections {
		DrawText(dst, sec.Label, SectionPadding, y, FontSizeHeading, ColorPrimary)
		y += FontSizeHeading + 8

		for ii, item := range sec.Items {
			isFocused := si == ss.sectionIndex && ii == ss.itemIndex
			rowH := float32(40)

			if isFocused {
				vector.DrawFilledRect(dst, float32(SectionPadding-8), float32(y-4),
					float32(ScreenWidth-SectionPadding*2+16), rowH, ColorSurfaceHover, false)
			}

			labelColor := ColorTextSecondary
			if isFocused {
				labelColor = ColorText
			}
			DrawText(dst, item.Label, SectionPadding, y+4, FontSizeBody, labelColor)

			value := item.Value()
			if ss.editing && isFocused {
				value = ss.editValue + "│"
			}
			DrawText(dst, value, SectionPadding+300, y+4, FontSizeBody, ColorTextSecondary)

			y += float64(rowH)
		}
		y += 16
	}

	// Save hint
	DrawText(dst, "Esc to go back (auto-saves)", SectionPadding, float64(ScreenHeight)-40,
		FontSizeSmall, ColorTextMuted)
}
