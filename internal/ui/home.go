package ui

import (
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

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
	scrollY      float64
	targetScrollY float64

	// Callbacks
	OnItemSelected func(item jellyfin.MediaItem)
	OnSearch       func()

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

	// Continue Watching
	if items, err := hs.client.GetResumeItems(20); err == nil && len(items) > 0 {
		grid := NewPosterGrid("Continue Watching")
		grid.Items = hs.convertItems(items)
		sections = append(sections, grid)
	}

	// Next Up
	if items, err := hs.client.GetNextUp(20); err == nil && len(items) > 0 {
		grid := NewPosterGrid("Next Up")
		grid.Items = hs.convertItems(items)
		sections = append(sections, grid)
	}

	// Libraries
	views, err := hs.client.GetViews()
	if err != nil {
		log.Printf("Failed to load views: %v", err)
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
			grid.Items = hs.convertItems(items)
			sections = append(sections, grid)
		}
	}

	hs.mu.Lock()
	hs.sections = sections
	if len(sections) > 0 {
		sections[0].Active = true
	}
	hs.loaded = true
	hs.loading = false
	hs.mu.Unlock()
}

func (hs *HomeScreen) convertItems(items []jellyfin.MediaItem) []GridItem {
	result := make([]GridItem, len(items))
	for i, item := range items {
		result[i] = GridItem{
			ID:    item.ID,
			Title: item.Name,
		}
		if item.Year > 0 {
			result[i].Subtitle = string(rune('0'+item.Year/1000)) +
				string(rune('0'+(item.Year/100)%10)) +
				string(rune('0'+(item.Year/10)%10)) +
				string(rune('0'+item.Year%10))
		}

		// Async load poster image
		url := hs.client.GetPosterURL(item.ID)
		idx := i
		hs.imgCache.LoadAsync(url, func(img *ebiten.Image) {
			hs.mu.Lock()
			defer hs.mu.Unlock()
			// Check bounds since sections may have changed
			for _, sec := range hs.sections {
				if idx < len(sec.Items) && sec.Items[idx].ID == items[idx].ID {
					sec.Items[idx].Image = img
					break
				}
			}
		})

		// Also check if already cached
		if img := hs.imgCache.Get(url); img != nil {
			result[i].Image = img
		}
	}
	return result
}

func (hs *HomeScreen) Update() (*ScreenTransition, error) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !hs.loaded || len(hs.sections) == 0 {
		return nil, nil
	}

	dir, enter, _ := InputState()

	// Search shortcut
	if ebiten.IsKeyPressed(ebiten.KeySlash) {
		if hs.OnSearch != nil {
			hs.OnSearch()
		}
		return nil, nil
	}

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

	if len(hs.sections) == 0 {
		DrawTextCentered(dst, "No media found", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	// Header
	DrawText(dst, "JellyCouch", SectionPadding, 16, FontSizeTitle, ColorPrimary)
	DrawText(dst, "/ to search", float64(ScreenWidth)-200, 24, FontSizeSmall, ColorTextMuted)

	y := float64(NavBarHeight+10) - hs.scrollY
	for _, section := range hs.sections {
		h := section.Draw(dst, SectionPadding, y)
		y += h + SectionGap
	}
}
