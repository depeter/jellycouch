package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
)

// HomeScreen displays library sections: Continue Watching, Next Up, and each library's latest items.
type HomeScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache

	sections     []*PosterGrid
	sectionIndex int
	loaded       bool
	loading      bool
	loadError    string
	scrollY      float64
	targetScrollY float64

	// Callbacks
	OnItemSelected func(item jellyfin.MediaItem)
	OnSearch       func()
	OnSettings     func()

	mu sync.Mutex
}

func NewHomeScreen(client *jellyfin.Client, imgCache *cache.ImageCache) *HomeScreen {
	return &HomeScreen{
		client:   client,
		imgCache: imgCache,
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

	// Settings shortcut
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if hs.OnSettings != nil {
			hs.OnSettings()
		}
		return nil, nil
	}

	// Search shortcut — just pressed only
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		if hs.OnSearch != nil {
			hs.OnSearch()
		}
		return nil, nil
	}

	// Mouse wheel scroll
	_, wy := MouseWheelDelta()
	if wy != 0 {
		hs.targetScrollY -= wy * 60
		if hs.targetScrollY < 0 {
			hs.targetScrollY = 0
		}
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && hs.loaded && len(hs.sections) > 0 {
		for i, section := range hs.sections {
			if idx, ok := section.HandleClick(mx, my); ok {
				// Set active section
				hs.sections[hs.sectionIndex].Active = false
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
	DrawText(dst, "/ search  S settings", float64(ScreenWidth)-300, 24, FontSizeSmall, ColorTextMuted)

	y := float64(NavBarHeight+10) - hs.scrollY
	for _, section := range hs.sections {
		h := section.Draw(dst, SectionPadding, y)
		y += h + SectionGap
	}
}
