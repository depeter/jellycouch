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

	sections     []settingsSection
	sectionIndex int
	itemIndex    int
	editing      bool
	editInput    TextInput
	editError    string

	// Scroll offset for long settings lists
	scrollY       float64
	targetScrollY float64

	// Row rects for mouse clicks (flat list across all sections)
	rowRects []settingsRowRect
	// Paste button rect (only valid while editing)
	pasteRect    ButtonRect
	pendingPaste chan string // async clipboard result for paste button

	langEditor  *LangEditor
	keyCapture  *KeyCaptureOverlay

	OnSave    func()
	OnSignOut func()
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
	Label      string
	Value      func() string
	OnChange   func(val string) error // returns error if validation fails
	Options    []string               // when set, Left/Right cycles through these instead of text edit
	MultiLang  bool                   // when set, Enter opens multi-language editor overlay
	Action     func()                 // when set, Enter/click invokes this; no value editor
	KeyCapture bool                   // when set, Enter opens key-capture overlay
}

var hwAccelOptions = []string{"auto-safe", "auto", "no", "vaapi", "vdpau", "cuda", "videotoolbox", "d3d11va", "dxva2"}

// boolOptions drives toggle-style settings items. The stored string is the
// user-visible value; Yes maps to true, No maps to false.
var boolOptions = []string{"Yes", "No"}

func boolToYN(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func ynToBool(s string) bool {
	return s == "Yes"
}

// uiScaleOptions are the presets surfaced in Settings. Keep in sync with
// config.DisplayConfig clamp range (0.5–2.0).
var uiScaleOptions = []string{"0.75", "1.00", "1.25", "1.50", "1.75", "2.00"}

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
				{Label: "Sign out", Value: func() string { return "" }, Action: func() {
					if ss.OnSignOut != nil {
						ss.OnSignOut()
					}
				}},
			},
		},
		{
			Label: "Jellyseerr",
			Items: []settingsItem{
				{Label: "URL", Value: func() string { return cfg.Jellyseerr.URL }, OnChange: func(v string) error { cfg.Jellyseerr.URL = v; return nil }},
				{Label: "API Key", Value: func() string { return cfg.Jellyseerr.APIKey }, OnChange: func(v string) error { cfg.Jellyseerr.APIKey = v; return nil }},
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
			Label: "Subtitle Providers",
			Items: []settingsItem{
				{Label: "OpenSubtitles: Enabled", Value: func() string { return boolToYN(cfg.SubtitleProviders.OpenSubtitles.Enabled) }, OnChange: func(v string) error { cfg.SubtitleProviders.OpenSubtitles.Enabled = ynToBool(v); return nil }, Options: boolOptions},
				{Label: "OpenSubtitles: API Key", Value: func() string { return cfg.SubtitleProviders.OpenSubtitles.APIKey }, OnChange: func(v string) error { cfg.SubtitleProviders.OpenSubtitles.APIKey = v; return nil }},
				{Label: "OpenSubtitles: Username", Value: func() string { return cfg.SubtitleProviders.OpenSubtitles.Username }, OnChange: func(v string) error { cfg.SubtitleProviders.OpenSubtitles.Username = v; return nil }},
				{Label: "OpenSubtitles: Password", Value: func() string { return cfg.SubtitleProviders.OpenSubtitles.Password }, OnChange: func(v string) error { cfg.SubtitleProviders.OpenSubtitles.Password = v; return nil }},
				{Label: "Subdl: Enabled", Value: func() string { return boolToYN(cfg.SubtitleProviders.Subdl.Enabled) }, OnChange: func(v string) error { cfg.SubtitleProviders.Subdl.Enabled = ynToBool(v); return nil }, Options: boolOptions},
				{Label: "Subdl: API Key", Value: func() string { return cfg.SubtitleProviders.Subdl.APIKey }, OnChange: func(v string) error { cfg.SubtitleProviders.Subdl.APIKey = v; return nil }},
			},
		},
		{
			Label: "Playback",
			Items: []settingsItem{
				{Label: "HW Accel", Value: func() string { return cfg.Playback.HWAccel }, OnChange: func(v string) error { cfg.Playback.HWAccel = v; return nil }, Options: hwAccelOptions},
				{Label: "Audio Language", Value: func() string { return cfg.Playback.AudioLanguage }, OnChange: func(v string) error { cfg.Playback.AudioLanguage = v; return nil }, MultiLang: true},
				{Label: "Sub Language", Value: func() string { return cfg.Playback.SubLanguage }, OnChange: func(v string) error { cfg.Playback.SubLanguage = v; return nil }, MultiLang: true},
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
		{
			Label: "Display",
			Items: []settingsItem{
				{Label: "UI Scale", Value: func() string { return fmt.Sprintf("%.2f", cfg.Display.UIScale) }, OnChange: func(v string) error {
					f, err := strconv.ParseFloat(v, 64)
					if err != nil {
						return fmt.Errorf("invalid number: %s", v)
					}
					cfg.Display.UIScale = f
					return nil
				}, Options: uiScaleOptions},
			},
		},
		{
			Label: "Keybindings",
			Items: []settingsItem{
				{Label: "Play / Pause", Value: func() string { return cfg.Keybindings.PlayPause }, OnChange: func(v string) error { cfg.Keybindings.PlayPause = v; return nil }, KeyCapture: true},
				{Label: "Seek -10s", Value: func() string { return cfg.Keybindings.SeekSmallBack }, OnChange: func(v string) error { cfg.Keybindings.SeekSmallBack = v; return nil }, KeyCapture: true},
				{Label: "Seek +10s", Value: func() string { return cfg.Keybindings.SeekSmallForward }, OnChange: func(v string) error { cfg.Keybindings.SeekSmallForward = v; return nil }, KeyCapture: true},
				{Label: "Seek -60s", Value: func() string { return cfg.Keybindings.SeekLargeBack }, OnChange: func(v string) error { cfg.Keybindings.SeekLargeBack = v; return nil }, KeyCapture: true},
				{Label: "Seek +60s", Value: func() string { return cfg.Keybindings.SeekLargeForward }, OnChange: func(v string) error { cfg.Keybindings.SeekLargeForward = v; return nil }, KeyCapture: true},
				{Label: "Volume Up", Value: func() string { return cfg.Keybindings.VolumeUp }, OnChange: func(v string) error { cfg.Keybindings.VolumeUp = v; return nil }, KeyCapture: true},
				{Label: "Volume Down", Value: func() string { return cfg.Keybindings.VolumeDown }, OnChange: func(v string) error { cfg.Keybindings.VolumeDown = v; return nil }, KeyCapture: true},
				{Label: "Mute", Value: func() string { return cfg.Keybindings.Mute }, OnChange: func(v string) error { cfg.Keybindings.Mute = v; return nil }, KeyCapture: true},
				{Label: "Cycle Subtitles", Value: func() string { return cfg.Keybindings.CycleSubtitles }, OnChange: func(v string) error { cfg.Keybindings.CycleSubtitles = v; return nil }, KeyCapture: true},
				{Label: "Cycle Audio", Value: func() string { return cfg.Keybindings.CycleAudio }, OnChange: func(v string) error { cfg.Keybindings.CycleAudio = v; return nil }, KeyCapture: true},
				{Label: "Show Info / Overlay", Value: func() string { return cfg.Keybindings.ShowInfo }, OnChange: func(v string) error { cfg.Keybindings.ShowInfo = v; return nil }, KeyCapture: true},
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

// focusedItem returns the currently focused settings item.
func (ss *SettingsScreen) focusedItem() *settingsItem {
	return &ss.sections[ss.sectionIndex].Items[ss.itemIndex]
}

// cycleOption moves to the next or previous option for an Options item.
func cycleOption(item *settingsItem, delta int) {
	current := item.Value()
	idx := -1
	for i, opt := range item.Options {
		if opt == current {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	} else {
		idx += delta
		if idx < 0 {
			idx = len(item.Options) - 1
		} else if idx >= len(item.Options) {
			idx = 0
		}
	}
	item.OnChange(item.Options[idx])
}

// itemY computes the Y position of a settings item (before scroll offset).
func (ss *SettingsScreen) itemY(si, ii int) float64 {
	y := float64(NavBarHeight*2 + 10)
	for s := 0; s <= si; s++ {
		y += FontSizeHeading + 12 // section heading
		if s < si {
			y += float64(len(ss.sections[s].Items)) * 64
			y += 24 // section gap
		} else {
			y += float64(ii) * 64
		}
	}
	return y
}

// ensureFocusVisible adjusts scroll so the focused item is on screen.
func (ss *SettingsScreen) ensureFocusVisible() {
	itemTop := ss.itemY(ss.sectionIndex, ss.itemIndex)
	itemBottom := itemTop + 64
	viewTop := ss.targetScrollY
	viewBottom := ss.targetScrollY + float64(ScreenHeight-NavBarHeight)

	if itemBottom > viewBottom {
		ss.targetScrollY = itemBottom - float64(ScreenHeight-NavBarHeight)
	}
	if itemTop < viewTop {
		ss.targetScrollY = itemTop
	}
	if ss.targetScrollY < 0 {
		ss.targetScrollY = 0
	}
}

func (ss *SettingsScreen) openLangEditor(item *settingsItem) {
	title := item.Label + " Preferences"
	ss.langEditor = NewLangEditor(title, item.Value())
}

func (ss *SettingsScreen) openKeyCapture(item *settingsItem) {
	ss.keyCapture = NewKeyCaptureOverlay(item.Label, item.Value())
}

func (ss *SettingsScreen) Update() (*ScreenTransition, error) {
	// Delegate to lang editor overlay when active
	if ss.langEditor != nil {
		ss.langEditor.Update()
		if done, result := ss.langEditor.Done(); done {
			ss.focusedItem().OnChange(result)
			ss.langEditor = nil
		}
		return nil, nil
	}

	// Delegate to key-capture overlay when active
	if ss.keyCapture != nil {
		ss.keyCapture.Update()
		if done, result, canceled := ss.keyCapture.Done(); done {
			if !canceled && result != "" {
				ss.focusedItem().OnChange(result)
			}
			ss.keyCapture = nil
		}
		return nil, nil
	}

	_, enter, back := InputState()

	if ss.editing {
		if ss.editInput.Update() {
			ss.editError = "" // clear error as user types
		}
		// Check for pending paste result
		if ss.pendingPaste != nil {
			select {
			case clip := <-ss.pendingPaste:
				ss.pendingPaste = nil
				if clip != "" {
					ss.editInput.insertAtCursor(clip)
					ss.editError = ""
				}
			default:
			}
		}
		// Paste button click — start async clipboard read
		mx, my, clicked := MouseJustClicked()
		if clicked && PointInRect(mx, my, ss.pasteRect.X, ss.pasteRect.Y, ss.pasteRect.W, ss.pasteRect.H) {
			if ss.pendingPaste == nil {
				ch := make(chan string, 1)
				ss.pendingPaste = ch
				go func() { ch <- readClipboard() }()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			// Apply edit with validation
			item := ss.focusedItem()
			if err := item.OnChange(ss.editInput.Text); err != nil {
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

	// Mouse wheel scrolling
	_, wy := MouseWheelDelta()
	if wy != 0 {
		ss.targetScrollY -= wy * ScrollWheelSpeed
		if ss.targetScrollY < 0 {
			ss.targetScrollY = 0
		}
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
				item := ss.focusedItem()
				if item.Action != nil {
					item.Action()
				} else if item.KeyCapture {
					ss.openKeyCapture(item)
				} else if item.MultiLang {
					ss.openLangEditor(item)
				} else if item.Options != nil {
					// Cycle forward on click
					cycleOption(item, 1)
				} else {
					ss.editInput = NewTextInput(item.Value())
					ss.editing = true
					ss.editError = ""
				}
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
				// Focus navbar when at the very top
				return &ScreenTransition{Type: TransitionFocusNavBar}, nil
			} else {
				ss.itemIndex = len(ss.sections[ss.sectionIndex].Items) - 1
			}
		}
		ss.ensureFocusVisible()
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
		ss.ensureFocusVisible()
	case DirLeft:
		item := ss.focusedItem()
		if item.Options != nil {
			cycleOption(item, -1)
		}
	case DirRight:
		item := ss.focusedItem()
		if item.Options != nil {
			cycleOption(item, 1)
		}
	}

	if enter {
		item := ss.focusedItem()
		if item.Action != nil {
			item.Action()
		} else if item.KeyCapture {
			ss.openKeyCapture(item)
		} else if item.MultiLang {
			ss.openLangEditor(item)
		} else if item.Options != nil {
			// Cycle forward on Enter for option items
			cycleOption(item, 1)
		} else {
			ss.editInput = NewTextInput(item.Value())
			ss.editing = true
			ss.editError = ""
		}
	}

	return nil, nil
}

func (ss *SettingsScreen) Draw(dst *ebiten.Image) {
	// Smooth scroll animation
	ss.scrollY = Lerp(ss.scrollY, ss.targetScrollY, ScrollAnimSpeed)

	DrawText(dst, "Settings", SectionPadding, NavBarHeight+20-ss.scrollY, FontSizeTitle, ColorText)

	y := float64(NavBarHeight*2+10) - ss.scrollY
	ss.rowRects = ss.rowRects[:0] // reset

	for si, sec := range ss.sections {
		DrawText(dst, sec.Label, SectionPadding, y, FontSizeHeading, ColorPrimary)
		y += FontSizeHeading + 12

		for ii, item := range sec.Items {
			isFocused := si == ss.sectionIndex && ii == ss.itemIndex
			rowH := float32(64)
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
			DrawText(dst, item.Label, SectionPadding, y+8, FontSizeBody, labelColor)

			valueX := SectionPadding + 400.0
			value := item.Value()
			isEditing := ss.editing && isFocused

			if isEditing {
				value = ss.editInput.DisplayText()
				// Blue border around value field when editing
				vx := float32(valueX - 4)
				vw := float32(rowW) - float32(400) - 8
				vector.StrokeRect(dst, vx, float32(y-2), vw, float32(rowH)-4, 2, ColorFocusBorder, false)
				// Paste button at the right end of the edit field
				pasteW := 80.0
				pasteH := float64(rowH) - 8
				pasteX := float64(vx+vw) - pasteW - 4
				pasteY := y - 1
				ss.pasteRect = ButtonRect{X: pasteX, Y: pasteY, W: pasteW, H: pasteH}
				vector.DrawFilledRect(dst, float32(pasteX), float32(pasteY), float32(pasteW), float32(pasteH), ColorSurface, false)
				vector.StrokeRect(dst, float32(pasteX), float32(pasteY), float32(pasteW), float32(pasteH), 1, ColorTextMuted, false)
				DrawTextCentered(dst, "Paste", pasteX+pasteW/2, pasteY+pasteH/2, FontSizeSmall, ColorTextSecondary)
			}

			if item.MultiLang && isFocused && !isEditing {
				// Multi-language item: show display names and edit hint
				display := formatLangDisplay(value)
				if display == "" {
					display = "(none)"
				}
				DrawText(dst, display, valueX, y+8, FontSizeBody, ColorText)
				w, _ := MeasureText(display, FontSizeBody)
				DrawText(dst, "[Enter to edit]", valueX+w+16, y+8, FontSizeSmall, ColorPrimary)
			} else if item.MultiLang {
				// Multi-language item (not focused): show display names
				display := formatLangDisplay(value)
				if display == "" {
					display = "(none)"
				}
				DrawText(dst, display, valueX, y+8, FontSizeBody, ColorTextSecondary)
			} else if item.Action != nil {
				hint := "[Enter]"
				color := ColorTextSecondary
				if isFocused {
					color = ColorPrimary
				}
				DrawText(dst, hint, valueX, y+8, FontSizeBody, color)
			} else if item.KeyCapture {
				display := value
				if display == "" {
					display = "(unbound)"
				}
				valueColor := ColorTextSecondary
				if isFocused {
					valueColor = ColorText
				}
				DrawText(dst, display, valueX, y+8, FontSizeBody, valueColor)
				if isFocused {
					w, _ := MeasureText(display, FontSizeBody)
					DrawText(dst, "[Enter to rebind]", valueX+w+16, y+8, FontSizeSmall, ColorPrimary)
				}
			} else if item.Options != nil && isFocused && !isEditing {
				// Draw arrows around value for cycle-able items
				DrawText(dst, "◀", valueX-28, y+8, FontSizeBody, ColorPrimary)
				DrawText(dst, value, valueX, y+8, FontSizeBody, ColorText)
				w, _ := MeasureText(value, FontSizeBody)
				DrawText(dst, "▶", valueX+w+12, y+8, FontSizeBody, ColorPrimary)
			} else {
				valueColor := ColorTextSecondary
				if isFocused && !isEditing {
					valueColor = ColorText
				}
				DrawText(dst, value, valueX, y+8, FontSizeBody, valueColor)
			}

			// Show edit error below the row
			if isEditing && ss.editError != "" {
				DrawText(dst, ss.editError, valueX, y+float64(rowH)-4, FontSizeSmall, ColorError)
			}

			y += float64(rowH)
		}
		y += 24
	}

	// Draw lang editor overlay on top
	if ss.langEditor != nil {
		ss.langEditor.Draw(dst)
	}
	if ss.keyCapture != nil {
		ss.keyCapture.Draw(dst)
	}
}
