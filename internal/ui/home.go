package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/jellyseerr"
)

// HomeScreen displays library sections: Continue Watching, Next Up, and each library's latest items.
type HomeScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache

	sections     []*PosterGrid
	sectionIndex int
	focusMode    int // 0=search bar, 1=sections
	input        TextInput
	loaded       bool
	loading      bool
	loadError    string
	scrollY      float64
	targetScrollY float64

	// Callbacks
	OnItemSelected func(item jellyfin.MediaItem)
	OnSearch       func(query string)
	OnSettings     func()
	OnRequests     func()

	// Jellyseerr client (nil if not configured)
	JellyseerrClient *jellyseerr.Client

	mu sync.Mutex
}

func NewHomeScreen(client *jellyfin.Client, imgCache *cache.ImageCache) *HomeScreen {
	return &HomeScreen{
		client:    client,
		imgCache:  imgCache,
		focusMode: 1, // start with sections focused
	}
}

func (hs *HomeScreen) Name() string { return "Home" }

func (hs *HomeScreen) OnEnter() {
	if !hs.loaded && !hs.loading {
		hs.loading = true
		go hs.loadData()
	}
}

func (hs *HomeScreen) OnExit() {}

func (hs *HomeScreen) loadData() {
	var sections []*PosterGrid
	var anyError error

	// Continue Watching
	if items, err := hs.client.GetResumeItems(20); err == nil && len(items) > 0 {
		grid := NewPosterGrid("Continue Watching")
		hs.convertItemsForGrid(grid, items)
		sections = append(sections, grid)
	} else if err != nil {
		anyError = err
	}

	// Next Up
	if items, err := hs.client.GetNextUp(20); err == nil && len(items) > 0 {
		grid := NewPosterGrid("Next Up")
		hs.convertItemsForGrid(grid, items)
		sections = append(sections, grid)
	} else if err != nil {
		anyError = err
	}

	// Libraries
	views, err := hs.client.GetViews()
	if err != nil {
		log.Printf("Failed to load views: %v", err)
		anyError = err
	} else {
		for _, view := range views {
			items, err := hs.client.GetLatestMedia(view.ID, 20)
			if err != nil {
				log.Printf("Failed to load latest for %s: %v", view.Name, err)
				continue
			}
			if len(items) == 0 {
				continue
			}
			grid := NewPosterGrid("Latest " + view.Name)
			hs.convertItemsForGrid(grid, items)
			sections = append(sections, grid)
		}
	}

	hs.mu.Lock()
	hs.sections = sections
	if len(sections) > 0 {
		sections[0].Active = true
	}
	if len(sections) == 0 && anyError != nil {
		hs.loadError = "Failed to load: " + anyError.Error()
	}
	hs.loaded = true
	hs.loading = false
	hs.mu.Unlock()
}

func (hs *HomeScreen) convertItemsForGrid(grid *PosterGrid, items []jellyfin.MediaItem) {
	result := make([]GridItem, len(items))
	for i, item := range items {
		result[i] = GridItem{
			ID:    item.ID,
			Title: item.Name,
		}
		if item.Year > 0 {
			result[i].Subtitle = fmt.Sprintf("%d", item.Year)
		}

		// Flow progress and watched state
		if item.RuntimeTicks > 0 && item.PlaybackPositionTicks > 0 {
			result[i].Progress = float64(item.PlaybackPositionTicks) / float64(item.RuntimeTicks)
		}
		result[i].Watched = item.Played

		// Async load poster image — capture grid pointer and item ID for race-safe callback
		url := hs.client.GetPosterURL(item.ID)
		itemID := item.ID
		hs.imgCache.LoadAsync(url, func(img *ebiten.Image) {
			hs.mu.Lock()
			defer hs.mu.Unlock()
			for j := range grid.Items {
				if grid.Items[j].ID == itemID {
					grid.Items[j].Image = img
					break
				}
			}
		})

		// Also check if already cached
		if img := hs.imgCache.Get(url); img != nil {
			result[i].Image = img
		}
	}
	grid.Items = result
}

func (hs *HomeScreen) Update() (*ScreenTransition, error) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	// Mouse wheel scroll (always active)
	_, wy := MouseWheelDelta()
	if wy != 0 {
		hs.targetScrollY -= wy * 60
		if hs.targetScrollY < 0 {
			hs.targetScrollY = 0
		}
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked {
		// Search bar click — focus the search bar for typing
		searchX := float64(ScreenWidth)/2 - 200
		searchY := 12.0
		searchW := 400.0
		searchH := 38.0
		if PointInRect(mx, my, searchX, searchY, searchW, searchH) {
			if hs.focusMode == 1 && len(hs.sections) > 0 {
				hs.sections[hs.sectionIndex].Active = false
			}
			hs.focusMode = 0
			return nil, nil
		}
		// Settings button click
		settingsX := float64(ScreenWidth) - SectionPadding - 80
		settingsY := 14.0
		settingsW := 80.0
		settingsH := 34.0
		if PointInRect(mx, my, settingsX, settingsY, settingsW, settingsH) {
			if hs.OnSettings != nil {
				hs.OnSettings()
			}
			return nil, nil
		}
		// Requests button click (only when Jellyseerr is configured)
		if hs.JellyseerrClient != nil {
			reqX := settingsX - 100
			reqY := 14.0
			reqW := 90.0
			reqH := 34.0
			if PointInRect(mx, my, reqX, reqY, reqW, reqH) {
				if hs.OnRequests != nil {
					hs.OnRequests()
				}
				return nil, nil
			}
		}
	}
	if clicked && hs.loaded && len(hs.sections) > 0 {
		for i, section := range hs.sections {
			if idx, ok := section.HandleClick(mx, my); ok {
				// Set active section
				if hs.focusMode == 1 {
					hs.sections[hs.sectionIndex].Active = false
				}
				hs.focusMode = 1
				hs.sectionIndex = i
				hs.sections[hs.sectionIndex].Active = true
				section.Focused = idx

				// Select the item
				item := section.SelectedItem()
				if item != nil && hs.OnItemSelected != nil {
					fullItem, err := hs.client.GetItem(item.ID)
					if err == nil {
						hs.OnItemSelected(*fullItem)
					}
				}
				return nil, nil
			}
		}
	}

	// Right-click: toggle watched state
	rmx, rmy, rclicked := MouseJustRightClicked()
	if rclicked && hs.loaded && len(hs.sections) > 0 {
		for _, section := range hs.sections {
			if idx, ok := section.HandleClick(rmx, rmy); ok {
				item := &section.Items[idx]
				if item.Watched {
					go hs.client.MarkUnplayed(item.ID)
				} else {
					go hs.client.MarkPlayed(item.ID)
				}
				item.Watched = !item.Watched
				return nil, nil
			}
		}
	}

	// Search bar focused — handle text input
	if hs.focusMode == 0 {
		hs.input.Update()

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && hs.input.Text != "" {
			query := hs.input.Text
			hs.input.Clear()
			hs.focusMode = 1
			if len(hs.sections) > 0 {
				hs.sections[hs.sectionIndex].Active = true
			}
			if hs.OnSearch != nil {
				hs.OnSearch(query)
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			hs.focusMode = 1
			if len(hs.sections) > 0 {
				hs.sections[hs.sectionIndex].Active = true
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && hs.loaded && len(hs.sections) > 0 {
			hs.focusMode = 1
			hs.sections[hs.sectionIndex].Active = true
		}

		return nil, nil
	}

	// Keyboard shortcuts (only when sections focused, not search bar)
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if hs.OnSettings != nil {
			hs.OnSettings()
		}
		return nil, nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyR) && hs.JellyseerrClient != nil {
		if hs.OnRequests != nil {
			hs.OnRequests()
		}
		return nil, nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		if len(hs.sections) > 0 {
			hs.sections[hs.sectionIndex].Active = false
		}
		hs.focusMode = 0
		return nil, nil
	}

	if !hs.loaded || len(hs.sections) == 0 {
		return nil, nil
	}

	dir, enter, _ := InputState()

	currentSection := hs.sections[hs.sectionIndex]

	switch dir {
	case DirUp:
		if hs.sectionIndex > 0 {
			currentSection.Active = false
			hs.sectionIndex--
			hs.sections[hs.sectionIndex].Active = true
			hs.ensureSectionVisible()
		} else {
			currentSection.Active = false
			hs.focusMode = 0
		}
	case DirDown:
		if hs.sectionIndex < len(hs.sections)-1 {
			currentSection.Active = false
			hs.sectionIndex++
			hs.sections[hs.sectionIndex].Active = true
			hs.ensureSectionVisible()
		}
	case DirLeft, DirRight:
		currentSection.Update(dir)
	}

	if enter {
		item := currentSection.SelectedItem()
		if item != nil && hs.OnItemSelected != nil {
			// Fetch full item data
			fullItem, err := hs.client.GetItem(item.ID)
			if err == nil {
				hs.OnItemSelected(*fullItem)
			}
		}
	}

	return nil, nil
}

func (hs *HomeScreen) ensureSectionVisible() {
	sectionHeight := float64(PosterHeight + FontSizeSmall + 16 + PosterFocusPad*2 + SectionTitleH + SectionGap)
	targetY := float64(hs.sectionIndex) * sectionHeight
	maxScroll := targetY - float64(ScreenHeight)/4
	if maxScroll < 0 {
		maxScroll = 0
	}
	hs.targetScrollY = maxScroll
}

func (hs *HomeScreen) Draw(dst *ebiten.Image) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	// Smooth scroll
	hs.scrollY = Lerp(hs.scrollY, hs.targetScrollY, ScrollAnimSpeed)

	if !hs.loaded {
		DrawTextCentered(dst, "Loading...", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	if hs.loadError != "" && len(hs.sections) == 0 {
		DrawTextCentered(dst, hs.loadError, float64(ScreenWidth)/2, float64(ScreenHeight)/2-20,
			FontSizeBody, ColorError)
		DrawTextCentered(dst, "Press Enter to retry", float64(ScreenWidth)/2, float64(ScreenHeight)/2+20,
			FontSizeSmall, ColorTextMuted)
		return
	}

	if len(hs.sections) == 0 {
		DrawTextCentered(dst, "No media found", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	// Header
	DrawText(dst, "JellyCouch", SectionPadding, 16, FontSizeTitle, ColorPrimary)

	// Search bar
	searchX := float64(ScreenWidth)/2 - 200
	searchY := 12.0
	searchW := 400.0
	searchH := 38.0
	if hs.focusMode == 0 {
		vector.DrawFilledRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), 2, ColorFocusBorder, false)
		if hs.input.Text == "" {
			DrawText(dst, "Search...", searchX+14, searchY+10, FontSizeBody, ColorTextMuted)
		}
		DrawText(dst, hs.input.DisplayText(), searchX+14, searchY+10, FontSizeBody, ColorText)
	} else {
		vector.DrawFilledRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), ColorSurface, false)
		vector.StrokeRect(dst, float32(searchX), float32(searchY), float32(searchW), float32(searchH), 1, ColorTextMuted, false)
		if hs.input.Text != "" {
			DrawText(dst, hs.input.Text, searchX+14, searchY+10, FontSizeBody, ColorText)
		} else {
			DrawText(dst, "\U0001F50D  Search library...", searchX+14, searchY+10, FontSizeBody, ColorTextMuted)
		}
	}

	// Requests button (only when Jellyseerr configured)
	settingsX := float64(ScreenWidth) - SectionPadding - 80
	if hs.JellyseerrClient != nil {
		reqX := settingsX - 100
		reqY := 14.0
		reqW := 90.0
		reqH := 34.0
		vector.DrawFilledRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), ColorSurface, false)
		DrawTextCentered(dst, "Requests", reqX+reqW/2, reqY+reqH/2, FontSizeSmall, ColorTextSecondary)
	}

	// Settings button
	settingsY := 14.0
	settingsW := 80.0
	settingsH := 34.0
	vector.DrawFilledRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), ColorSurface, false)
	DrawTextCentered(dst, "\u2699 Settings", settingsX+settingsW/2, settingsY+settingsH/2, FontSizeSmall, ColorTextSecondary)

	y := float64(NavBarHeight+10) - hs.scrollY
	for _, section := range hs.sections {
		h := section.Draw(dst, SectionPadding, y)
		y += h + SectionGap
	}
}
