package ui

import (
	"fmt"
	"log"
	"strings"
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
	episodesLoading bool

	// Season tab rects for mouse clicks
	seasonTabRects []ButtonRect
	// Episode rects for mouse clicks
	episodeRects []ButtonRect

	// Focus mode: 0=buttons, 1=episodes, 2=season tabs
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
		ds.detail.RatingValue = item.CommunityRating
	}
	ds.detail.Overview = item.Overview
	ds.detail.OfficialRating = item.OfficialRating
	if len(item.Genres) > 0 {
		ds.detail.Genres = strings.Join(item.Genres, ", ")
	}
	if len(item.Taglines) > 0 {
		ds.detail.Tagline = item.Taglines[0]
	}

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
	ds.mu.Lock()
	ds.episodesLoading = true
	ds.mu.Unlock()

	episodes, err := ds.client.GetEpisodes(ds.item.ID, seasonID)
	if err != nil {
		log.Printf("Failed to load episodes: %v", err)
		ds.mu.Lock()
		ds.episodesLoading = false
		ds.mu.Unlock()
		return
	}
	ds.mu.Lock()
	ds.episodes = episodes
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)
	ds.episodeGrid = NewFocusGrid(cols, len(episodes))
	ds.episodesLoading = false
	ds.mu.Unlock()
}

func (ds *DetailScreen) Update() (*ScreenTransition, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	dir, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked {
		// Check buttons
		if btnIdx, ok := ds.detail.HandleClick(mx, my); ok {
			ds.detail.ButtonIndex = btnIdx
			ds.handleButtonPress()
			return nil, nil
		}
		// Check season tabs
		for i, rect := range ds.seasonTabRects {
			if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
				if i != ds.selectedSeason {
					ds.selectedSeason = i
					ds.focusMode = 2
					go ds.loadEpisodes(ds.seasons[i].ID)
				}
				return nil, nil
			}
		}
		// Check episode items
		for i, rect := range ds.episodeRects {
			if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
				if ds.episodeGrid != nil {
					ds.episodeGrid.Focused = i
					ds.focusMode = 1
				}
				if i < len(ds.episodes) && ds.OnPlay != nil {
					ds.OnPlay(ds.episodes[i].ID, ds.episodes[i].PlaybackPositionTicks)
				}
				return nil, nil
			}
		}
	}

	// Right-click: toggle watched state on episodes
	rmx, rmy, rclicked := MouseJustRightClicked()
	if rclicked {
		for i, rect := range ds.episodeRects {
			if PointInRect(rmx, rmy, rect.X, rect.Y, rect.W, rect.H) {
				if i < len(ds.episodes) {
					if ds.episodes[i].Played {
						go ds.client.MarkUnplayed(ds.episodes[i].ID)
					} else {
						go ds.client.MarkPlayed(ds.episodes[i].ID)
					}
					ds.episodes[i].Played = !ds.episodes[i].Played
				}
				return nil, nil
			}
		}
	}

	switch ds.focusMode {
	case 0: // buttons
		if dir == DirUp {
			return &ScreenTransition{Type: TransitionFocusNavBar}, nil
		}
		if dir == DirDown {
			if len(ds.seasons) > 0 {
				ds.focusMode = 2
			} else if ds.episodeGrid != nil && len(ds.episodes) > 0 {
				ds.focusMode = 1
			}
		} else {
			ds.detail.Update(dir)
		}

		if enter {
			ds.handleButtonPress()
		}

	case 2: // season tabs
		switch dir {
		case DirUp:
			ds.focusMode = 0
		case DirDown:
			if ds.episodeGrid != nil && len(ds.episodes) > 0 {
				ds.focusMode = 1
			}
		case DirLeft:
			if ds.selectedSeason > 0 {
				ds.selectedSeason--
				go ds.loadEpisodes(ds.seasons[ds.selectedSeason].ID)
			}
		case DirRight:
			if ds.selectedSeason < len(ds.seasons)-1 {
				ds.selectedSeason++
				go ds.loadEpisodes(ds.seasons[ds.selectedSeason].ID)
			}
		}

	case 1: // episodes
		if dir == DirUp && ds.episodeGrid.FocusedRow() == 0 {
			if len(ds.seasons) > 0 {
				ds.focusMode = 2
			} else {
				ds.focusMode = 0
			}
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
	if len(ds.seasons) > 0 || (ds.episodeGrid != nil && len(ds.episodes) > 0) {
		y := float64(BackdropHeight + 250)

		// Season tabs
		if len(ds.seasons) > 0 {
			ds.seasonTabRects = make([]ButtonRect, len(ds.seasons))
			tabX := float64(SectionPadding)
			for i, season := range ds.seasons {
				label := season.Name
				if label == "" {
					label = fmt.Sprintf("Season %d", season.IndexNumber)
				}
				w, _ := MeasureText(label, FontSizeBody)
				tabH := FontSizeBody + 12.0

				ds.seasonTabRects[i] = ButtonRect{X: tabX - 4, Y: y - 4, W: w + 8, H: tabH}

				if i == ds.selectedSeason {
					if ds.focusMode == 2 {
						// Highlighted background when season tabs are focused
						vector.DrawFilledRect(dst, float32(tabX-8), float32(y-6),
							float32(w+16), float32(tabH+4), ColorSurfaceHover, false)
					}
					DrawText(dst, label, tabX, y, FontSizeBody, ColorPrimary)
					vector.DrawFilledRect(dst, float32(tabX-4), float32(y+FontSizeBody+2),
						float32(w+8), 2, ColorPrimary, false)
				} else {
					DrawText(dst, label, tabX, y, FontSizeBody, ColorTextMuted)
				}
				tabX += w + 24
			}
			y += FontSizeBody + 16
		}

		// Loading indicator
		if ds.episodesLoading {
			DrawTextCentered(dst, "Loading episodes...", float64(ScreenWidth)/2, y+50,
				FontSizeBody, ColorTextSecondary)
			return
		}

		// Focused episode overview
		if ds.focusMode == 1 && ds.episodeGrid != nil && ds.episodeGrid.Focused < len(ds.episodes) {
			ep := ds.episodes[ds.episodeGrid.Focused]
			epTitle := fmt.Sprintf("E%d: %s", ep.IndexNumber, ep.Name)
			DrawText(dst, epTitle, SectionPadding, y, FontSizeBody, ColorText)
			y += FontSizeBody + 4
			if ep.Overview != "" {
				maxW := float64(ScreenWidth) - SectionPadding*2 - 200
				h := DrawTextWrapped(dst, ep.Overview, SectionPadding, y, maxW, FontSizeSmall, ColorTextSecondary)
				y += h + 8
			}
		}

		// Episode items
		if ds.episodeGrid != nil && len(ds.episodes) > 0 {
			ds.episodeRects = make([]ButtonRect, len(ds.episodes))
			for i, ep := range ds.episodes {
				col := i % ds.episodeGrid.Cols
				row := i / ds.episodeGrid.Cols
				ex := SectionPadding + float64(col)*(PosterWidth+PosterGap)
				ey := y + float64(row)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16)

				ds.episodeRects[i] = ButtonRect{X: ex, Y: ey, W: PosterWidth, H: PosterHeight}

				isFocused := ds.focusMode == 1 && i == ds.episodeGrid.Focused
				title := fmt.Sprintf("E%d %s", ep.IndexNumber, ep.Name)

				// Flow progress for episode
				var progress float64
				if ep.RuntimeTicks > 0 && ep.PlaybackPositionTicks > 0 {
					progress = float64(ep.PlaybackPositionTicks) / float64(ep.RuntimeTicks)
				}

				gi := GridItem{
					ID:       ep.ID,
					Title:    title,
					Progress: progress,
					Watched:  ep.Played,
				}
				drawPosterItem(dst, gi, ex, ey, isFocused)
			}
		}
	}
}
