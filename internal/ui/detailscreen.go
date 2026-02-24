package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
)

// DetailScreen shows full item details with play options.
type DetailScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache
	item     jellyfin.MediaItem

	detail   *DetailPanel
	backdrop *ebiten.Image

	// For TV shows — season/episode list
	seasons  []jellyfin.MediaItem
	episodes []jellyfin.MediaItem
	episodeGrid *FocusGrid
	selectedSeason int

	// Focus mode: 0=buttons, 1=episodes
	focusMode  int
	loaded     bool

	OnPlay    func(itemID string, resumeTicks int64)
	OnLibrary func(parentID, title string)

	mu sync.Mutex
}

func NewDetailScreen(client *jellyfin.Client, imgCache *cache.ImageCache, item jellyfin.MediaItem) *DetailScreen {
	ds := &DetailScreen{
		client: client,
		imgCache: imgCache,
		item:   item,
		detail: NewDetailPanel(),
	}

	ds.detail.Title = item.Name
	if item.Year > 0 {
		ds.detail.Year = fmt.Sprintf("%d", item.Year)
	}
	if item.RuntimeTicks > 0 {
		ds.detail.Runtime = FormatRuntime(item.RuntimeTicks)
	}
	if item.CommunityRating > 0 {
		ds.detail.Rating = fmt.Sprintf("★ %.1f", item.CommunityRating)
	}
	ds.detail.Overview = item.Overview

	// Determine buttons
	buttons := []string{"Play"}
	if item.PlaybackPositionTicks > 0 {
		buttons = append([]string{"Resume"}, buttons...)
	}
	if item.Type == "Series" {
		buttons = append(buttons, "Browse Seasons")
	}
	buttons = append(buttons, toggleWatchedLabel(item.Played))
	ds.detail.Buttons = buttons

	return ds
}

func toggleWatchedLabel(played bool) string {
	if played {
		return "Mark Unwatched"
	}
	return "Mark Watched"
}

func (ds *DetailScreen) Name() string { return "Detail: " + ds.item.Name }

func (ds *DetailScreen) OnEnter() {
	go ds.loadBackdrop()
	if ds.item.Type == "Series" {
		go ds.loadSeasons()
	}
}

func (ds *DetailScreen) OnExit() {}

func (ds *DetailScreen) loadBackdrop() {
	url := ds.client.GetBackdropURL(ds.item.ID)
	ds.imgCache.LoadAsync(url, func(img *ebiten.Image) {
		ds.mu.Lock()
		ds.detail.Backdrop = img
		ds.mu.Unlock()
	})
}

func (ds *DetailScreen) loadSeasons() {
	seasons, err := ds.client.GetSeasons(ds.item.ID)
	if err != nil {
		log.Printf("Failed to load seasons: %v", err)
		return
	}
	ds.mu.Lock()
	ds.seasons = seasons
	ds.loaded = true
	ds.mu.Unlock()

	if len(seasons) > 0 {
		ds.loadEpisodes(seasons[0].ID)
	}
}

func (ds *DetailScreen) loadEpisodes(seasonID string) {
	episodes, err := ds.client.GetEpisodes(ds.item.ID, seasonID)
	if err != nil {
		log.Printf("Failed to load episodes: %v", err)
		return
	}
	ds.mu.Lock()
	ds.episodes = episodes
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)
	ds.episodeGrid = NewFocusGrid(cols, len(episodes))
	ds.mu.Unlock()
}

func (ds *DetailScreen) Update() (*ScreenTransition, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	dir, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	switch ds.focusMode {
	case 0: // buttons
		if dir == DirDown && ds.episodeGrid != nil && len(ds.episodes) > 0 {
			ds.focusMode = 1
		} else {
			ds.detail.Update(dir)
		}

		if enter {
			ds.handleButtonPress()
		}

	case 1: // episodes
		if dir == DirUp && ds.episodeGrid.FocusedRow() == 0 {
			ds.focusMode = 0
		} else if dir != DirNone {
			ds.episodeGrid.Update(dir)
		}

		if enter && ds.episodeGrid != nil {
			idx := ds.episodeGrid.Focused
			if idx < len(ds.episodes) {
				ep := ds.episodes[idx]
				if ds.OnPlay != nil {
					ds.OnPlay(ep.ID, ep.PlaybackPositionTicks)
				}
			}
		}
	}

	return nil, nil
}

func (ds *DetailScreen) handleButtonPress() {
	btn := ds.detail.Buttons[ds.detail.ButtonIndex]
	switch btn {
	case "Play":
		if ds.OnPlay != nil {
			ds.OnPlay(ds.item.ID, 0)
		}
	case "Resume":
		if ds.OnPlay != nil {
			ds.OnPlay(ds.item.ID, ds.item.PlaybackPositionTicks)
		}
	case "Browse Seasons":
		if ds.OnLibrary != nil {
			ds.OnLibrary(ds.item.ID, ds.item.Name)
		}
	case "Mark Watched":
		go ds.client.MarkPlayed(ds.item.ID)
		ds.item.Played = true
		ds.updateWatchedButton()
	case "Mark Unwatched":
		go ds.client.MarkUnplayed(ds.item.ID)
		ds.item.Played = false
		ds.updateWatchedButton()
	}
}

func (ds *DetailScreen) updateWatchedButton() {
	for i, btn := range ds.detail.Buttons {
		if btn == "Mark Watched" || btn == "Mark Unwatched" {
			ds.detail.Buttons[i] = toggleWatchedLabel(ds.item.Played)
			break
		}
	}
}

func (ds *DetailScreen) Draw(dst *ebiten.Image) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.detail.Draw(dst)

	// Episode list for TV shows
	if ds.episodeGrid != nil && len(ds.episodes) > 0 {
		y := float64(BackdropHeight + 250)

		// Season tabs
		if len(ds.seasons) > 0 {
			tabX := float64(SectionPadding)
			for i, season := range ds.seasons {
				label := season.Name
				if label == "" {
					label = fmt.Sprintf("Season %d", season.IndexNumber)
				}
				w, _ := MeasureText(label, FontSizeBody)
				clr := ColorTextMuted
				if i == ds.selectedSeason {
					clr = ColorPrimary
					vector.DrawFilledRect(dst, float32(tabX-4), float32(y+FontSizeBody+2),
						float32(w+8), 2, ColorPrimary, false)
				}
				DrawText(dst, label, tabX, y, FontSizeBody, clr)
				tabX += w + 24
			}
			y += FontSizeBody + 16
		}

		// Episode items
		for i, ep := range ds.episodes {
			col := i % ds.episodeGrid.Cols
			row := i / ds.episodeGrid.Cols
			ex := SectionPadding + float64(col)*(PosterWidth+PosterGap)
			ey := y + float64(row)*(PosterHeight+PosterGap+FontSizeCaption+8)

			isFocused := ds.focusMode == 1 && i == ds.episodeGrid.Focused
			title := fmt.Sprintf("E%d %s", ep.IndexNumber, ep.Name)
			gi := GridItem{ID: ep.ID, Title: title}
			drawPosterItem(dst, gi, ex, ey, isFocused)
		}
	}
}
