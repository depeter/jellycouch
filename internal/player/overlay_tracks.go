package player

import (
	"fmt"
	"strings"
	"time"
)

// TrackType distinguishes subtitle vs audio tracks.
type TrackType int

const (
	TrackSub   TrackType = iota
	TrackAudio
)

// Track holds metadata about an mpv track.
type Track struct {
	ID       int
	Type     TrackType
	Title    string
	Lang     string
	Codec    string
	Selected bool
	Default  bool
	Forced   bool
	External bool
}

// DisplayName returns a human-readable label for the track.
func (t Track) DisplayName() string {
	var parts []string

	name := t.Title
	if name == "" {
		name = langName(t.Lang)
	} else if lang := langName(t.Lang); lang != "" && lang != t.Title {
		name = t.Title + " - " + lang
	}
	if name == "" {
		name = fmt.Sprintf("Track %d", t.ID)
	}
	parts = append(parts, name)

	if t.Codec != "" {
		parts = append(parts, "["+t.Codec+"]")
	}

	var flags []string
	if t.Default {
		flags = append(flags, "default")
	}
	if t.Forced {
		flags = append(flags, "forced")
	}
	if t.External {
		flags = append(flags, "external")
	}
	if len(flags) > 0 {
		parts = append(parts, "("+strings.Join(flags, ", ")+")")
	}

	return strings.Join(parts, " ")
}

// OpenTrackPanel fetches tracks and opens the selection panel.
func (o *PlaybackOverlay) OpenTrackPanel(tt TrackType) {
	o.trackType = tt
	o.tracks = o.player.GetTracks(tt)
	o.selectedIndex = 0

	// Find the currently selected track to pre-focus it
	for i, t := range o.tracks {
		if t.Selected {
			o.selectedIndex = i
			break
		}
	}

	o.Mode = OverlayTrackSelect
	o.lastInput = time.Now()
	o.renderTrackPanel()
}

// HandleTrackInput handles input when the track selection panel is open.
// Returns true if input was consumed.
func (o *PlaybackOverlay) HandleTrackInput(dir Direction, enter, back bool) bool {
	o.lastInput = time.Now()

	if back {
		// Close panel, return to control bar
		o.Mode = OverlayBar
		o.renderBar()
		return true
	}

	totalItems := len(o.tracks)
	if o.trackType == TrackSub {
		totalItems++ // "Off" option
	}

	if dir == DirUp {
		if o.selectedIndex > 0 {
			o.selectedIndex--
		}
		o.renderTrackPanel()
		return true
	}

	if dir == DirDown {
		if o.selectedIndex < totalItems-1 {
			o.selectedIndex++
		}
		o.renderTrackPanel()
		return true
	}

	if enter {
		o.selectTrack()
		return true
	}

	// Consume all other input while modal is open
	return true
}

// selectTrack applies the selected track and closes the panel.
func (o *PlaybackOverlay) selectTrack() {
	if o.trackType == TrackSub {
		// Last item is "Off"
		if o.selectedIndex >= len(o.tracks) {
			o.player.SetSubTrack(0)
		} else {
			o.player.SetSubTrack(o.tracks[o.selectedIndex].ID)
		}
	} else {
		if o.selectedIndex < len(o.tracks) {
			o.player.SetAudioTrack(o.tracks[o.selectedIndex].ID)
		}
	}

	o.Mode = OverlayBar
	o.renderBar()
}

// renderTrackPanel renders the track selection panel ASS.
func (o *PlaybackOverlay) renderTrackPanel() {
	var b strings.Builder

	b.WriteString("${osd-ass-cc/0}")
	b.WriteString("{\\an5\\bord0\\shad0}")

	title := "Subtitle Tracks"
	if o.trackType == TrackAudio {
		title = "Audio Tracks"
	}
	b.WriteString(fmt.Sprintf("{\\fs%d\\bord1%s}", o.scale(15), assColorBlue) + title + "\\N\\N")

	totalItems := len(o.tracks)
	if o.trackType == TrackSub {
		totalItems++ // "Off" option at the end
	}

	for i := 0; i < totalItems; i++ {
		var label string
		var isCurrentlyActive bool

		if o.trackType == TrackSub && i >= len(o.tracks) {
			label = "Off"
			isCurrentlyActive = true
			for _, t := range o.tracks {
				if t.Selected {
					isCurrentlyActive = false
					break
				}
			}
		} else {
			t := o.tracks[i]
			label = t.DisplayName()
			isCurrentlyActive = t.Selected
		}

		b.WriteString(fmt.Sprintf("{\\fs%d\\bord1}", o.scale(13)))

		if i == o.selectedIndex {
			b.WriteString("{" + assColorBlue + "\\b1}")
			b.WriteString("\u25B8 " + label)
			b.WriteString("{\\b0}")
		} else if isCurrentlyActive {
			b.WriteString("{" + assColorWhite + "}")
			b.WriteString("\u2713 " + label)
		} else {
			b.WriteString("{" + assColorGray + "}")
			b.WriteString("   " + label)
		}

		b.WriteString("\\N")
	}

	o.player.ShowText(b.String(), 30000)
}

// langName converts ISO 639-2/B language codes to human-readable names.
func langName(code string) string {
	if code == "" {
		return ""
	}
	langs := map[string]string{
		"eng": "English",
		"fre": "French",
		"fra": "French",
		"spa": "Spanish",
		"ger": "German",
		"deu": "German",
		"ita": "Italian",
		"por": "Portuguese",
		"rus": "Russian",
		"jpn": "Japanese",
		"kor": "Korean",
		"chi": "Chinese",
		"zho": "Chinese",
		"ara": "Arabic",
		"hin": "Hindi",
		"tur": "Turkish",
		"pol": "Polish",
		"dut": "Dutch",
		"nld": "Dutch",
		"swe": "Swedish",
		"nor": "Norwegian",
		"dan": "Danish",
		"fin": "Finnish",
		"hun": "Hungarian",
		"ces": "Czech",
		"cze": "Czech",
		"rum": "Romanian",
		"ron": "Romanian",
		"gre": "Greek",
		"ell": "Greek",
		"heb": "Hebrew",
		"tha": "Thai",
		"vie": "Vietnamese",
		"ind": "Indonesian",
		"may": "Malay",
		"msa": "Malay",
		"ukr": "Ukrainian",
		"bul": "Bulgarian",
		"hrv": "Croatian",
		"srp": "Serbian",
		"slv": "Slovenian",
		"slk": "Slovak",
		"slo": "Slovak",
		"cat": "Catalan",
		"fil": "Filipino",
		"tam": "Tamil",
		"tel": "Telugu",
		"ben": "Bengali",
		"und": "Unknown",
	}
	if name, ok := langs[code]; ok {
		return name
	}
	return code
}
