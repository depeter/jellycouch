package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/opensubtitles"
)

// SubtitleDownloadScreen lets the user search and download external subtitle files.
type SubtitleDownloadScreen struct {
	client   *opensubtitles.Client
	item     jellyfin.MediaItem
	cacheDir string // parent dir for subtitle files

	results     []opensubtitles.SubtitleResult
	selectedIdx int
	loading     bool
	loadErr     string
	downloading bool
	downloadErr string
	downloadDone string // non-empty once download finishes

	OnSubtitleReady func(path string)
	OnCancel        func()

	mu sync.Mutex
}

// NewSubtitleDownloadScreen creates a new subtitle download screen.
func NewSubtitleDownloadScreen(client *opensubtitles.Client, item jellyfin.MediaItem, cacheDir string) *SubtitleDownloadScreen {
	return &SubtitleDownloadScreen{
		client:   client,
		item:     item,
		cacheDir: cacheDir,
	}
}

func (s *SubtitleDownloadScreen) Name() string { return "Subtitles: " + s.item.Name }

func (s *SubtitleDownloadScreen) OnEnter() {
	s.mu.Lock()
	s.loading = true
	s.loadErr = ""
	s.mu.Unlock()
	go s.loadResults()
}

func (s *SubtitleDownloadScreen) OnExit() {}

func (s *SubtitleDownloadScreen) loadResults() {
	params := s.buildSearchParams()
	resp, err := s.client.Search(params)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loading = false
	if err != nil {
		s.loadErr = err.Error()
		return
	}
	s.results = resp.Data
}

func (s *SubtitleDownloadScreen) buildSearchParams() opensubtitles.SearchParams {
	p := opensubtitles.SearchParams{}

	if imdb, ok := s.item.ProviderIds["Imdb"]; ok && imdb != "" {
		p.IMDBID = imdb
	} else {
		p.Query = s.item.Name
		p.Year = s.item.Year
	}

	switch s.item.Type {
	case "Movie":
		p.Type = "movie"
	case "Episode":
		p.Type = "episode"
		p.SeasonNumber = s.item.ParentIndexNumber
		p.EpisodeNumber = s.item.IndexNumber
	}

	return p
}

func (s *SubtitleDownloadScreen) downloadSelected() {
	s.mu.Lock()
	if s.downloading || s.selectedIdx >= len(s.results) {
		s.mu.Unlock()
		return
	}
	result := s.results[s.selectedIdx]
	if len(result.Attributes.Files) == 0 {
		s.mu.Unlock()
		return
	}
	file := result.Attributes.Files[0]
	s.downloading = true
	s.downloadErr = ""
	s.mu.Unlock()

	// Build a safe filename
	fname := file.FileName
	if fname == "" {
		fname = fmt.Sprintf("subtitle_%d.srt", file.FileID)
	}
	destPath := filepath.Join(s.cacheDir, fname)

	err := s.client.Download(file.FileID, destPath)
	s.mu.Lock()
	s.downloading = false
	if err != nil {
		s.downloadErr = err.Error()
	} else {
		s.downloadDone = destPath
	}
	s.mu.Unlock()

	if err == nil && s.OnSubtitleReady != nil {
		s.OnSubtitleReady(destPath)
	}
}

func (s *SubtitleDownloadScreen) Update() (*ScreenTransition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, enter, back := InputState()

	// If download is done, any key returns
	if s.downloadDone != "" {
		if enter || back {
			return &ScreenTransition{Type: TransitionPop}, nil
		}
		return nil, nil
	}

	if s.downloading || s.loading {
		if back {
			if s.OnCancel != nil {
				s.OnCancel()
			}
			return &ScreenTransition{Type: TransitionPop}, nil
		}
		return nil, nil
	}

	if back {
		if s.OnCancel != nil {
			s.OnCancel()
		}
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	if len(s.results) > 0 {
		dir, _, _ := InputState()
		switch dir {
		case DirUp:
			if s.selectedIdx > 0 {
				s.selectedIdx--
			}
		case DirDown:
			if s.selectedIdx < len(s.results)-1 {
				s.selectedIdx++
			}
		}

		// Mouse click
		mx, my, clicked := MouseJustClicked()
		if clicked {
			rowH := float64(FontSizeBody + 20)
			startY := float64(NavBarHeight*2 + 50)
			for i := range s.results {
				rowY := startY + float64(i)*rowH
				if PointInRect(mx, my, float64(SectionPadding), rowY, float64(ScreenWidth-SectionPadding*2), rowH) {
					s.selectedIdx = i
					go s.downloadSelected()
					return nil, nil
				}
			}
		}

		if enter {
			go s.downloadSelected()
		}
	}

	return nil, nil
}

func (s *SubtitleDownloadScreen) Draw(dst *ebiten.Image) {
	s.mu.Lock()
	defer s.mu.Unlock()

	title := "Download Subtitles: " + s.item.Name
	DrawText(dst, title, SectionPadding, NavBarHeight+16, FontSizeTitle, ColorText)

	y := float64(NavBarHeight*2 + 10)

	switch {
	case s.loading:
		DrawTextCentered(dst, "Searching...", float64(ScreenWidth)/2, y+60, FontSizeBody, ColorTextSecondary)

	case s.loadErr != "":
		DrawText(dst, "Error: "+s.loadErr, SectionPadding, y+20, FontSizeBody, ColorError)

	case s.downloading:
		DrawTextCentered(dst, "Downloading...", float64(ScreenWidth)/2, y+60, FontSizeBody, ColorPrimary)

	case s.downloadDone != "":
		fname := filepath.Base(s.downloadDone)
		DrawTextCentered(dst, "Ready: "+fname, float64(ScreenWidth)/2, y+40, FontSizeBody, ColorPrimary)
		DrawTextCentered(dst, "Press any key to continue", float64(ScreenWidth)/2, y+80, FontSizeSmall, ColorTextSecondary)

	case s.downloadErr != "":
		DrawText(dst, "Download error: "+s.downloadErr, SectionPadding, y+20, FontSizeBody, ColorError)

	case len(s.results) == 0:
		DrawTextCentered(dst, "No subtitles found", float64(ScreenWidth)/2, y+60, FontSizeBody, ColorTextSecondary)

	default:
		rowH := float64(FontSizeBody + 20)
		for i, result := range s.results {
			rowY := y + float64(i)*rowH
			isFocused := i == s.selectedIdx

			if isFocused {
				vector.DrawFilledRect(dst,
					float32(SectionPadding-8), float32(rowY-4),
					float32(ScreenWidth-SectionPadding*2+16), float32(rowH),
					ColorSurfaceHover, false)
			}

			// Language badge
			lang := strings.ToUpper(result.Attributes.Language)
			if lang == "" {
				lang = "??"
			}
			DrawText(dst, lang, SectionPadding, rowY+4, FontSizeSmall, ColorPrimary)

			// Release name
			relName := result.Attributes.ReleaseName
			if relName == "" {
				relName = "(unknown release)"
			}
			nameColor := ColorTextSecondary
			if isFocused {
				nameColor = ColorText
			}
			DrawText(dst, relName, SectionPadding+60, rowY+4, FontSizeBody, nameColor)

			// Flags and download count
			flags := []string{fmt.Sprintf("%d ↓", result.Attributes.DownloadCount)}
			if result.Attributes.HD {
				flags = append(flags, "HD")
			}
			if result.Attributes.HearingImpaired {
				flags = append(flags, "HI")
			}
			flagStr := strings.Join(flags, " · ")
			flagColor := ColorTextMuted
			if isFocused {
				flagColor = ColorTextSecondary
			}
			fw, _ := MeasureText(flagStr, FontSizeSmall)
			_ = fw
			DrawText(dst, flagStr, float64(ScreenWidth-SectionPadding)-200, rowY+4, FontSizeSmall, flagColor)
		}
	}
}
