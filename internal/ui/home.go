package ui

import (
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
)

// sectionMeta stores per-section library info. IsLibrary is true for library sections
// that support "See All" browsing.
type sectionMeta struct {
	IsLibrary bool
	ParentID  string
	Title     string
}

// HomeScreen displays library sections: Continue Watching, Next Up, and each library's latest items.
type HomeScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache

	sections     []*PosterGrid
	sectionIndex int
	loaded       bool
	loading      bool
	loadError    string
	ScrollState

	// Library views for nav buttons (still loaded for section labels)
	libraryViews []struct{ ID, Name string }

	// Per-section metadata for library browsing
	sectionMeta []sectionMeta

	// Callbacks
	OnItemSelected    func(item jellyfin.MediaItem)
	OnLibraryBrowse   func(parentID, title string)
	OnAuthError       func()

	authFailed bool
	errDisplay ErrorDisplay

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
	type sectionResult struct {
		grid  *PosterGrid
		meta  sectionMeta
		order int // for stable sort
	}

	var (
		resultsMu sync.Mutex
		results   []sectionResult
		anyError  error
		wg        sync.WaitGroup
	)

	addResult := func(sr sectionResult) {
		resultsMu.Lock()
		results = append(results, sr)
		resultsMu.Unlock()
	}

	setError := func(err error) {
		resultsMu.Lock()
		if anyError == nil {
			anyError = err
		}
		resultsMu.Unlock()
	}

	// Continue Watching (order 0)
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := hs.client.GetResumeItems(20)
		if err != nil {
			setError(err)
			return
		}
		if len(items) > 0 {
			grid := NewPosterGrid("Continue Watching")
			hs.convertItemsForGrid(grid, items)
			addResult(sectionResult{grid: grid, meta: sectionMeta{}, order: 0})
		}
	}()

	// Next Up (order 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := hs.client.GetNextUp(20)
		if err != nil {
			setError(err)
			return
		}
		if len(items) > 0 {
			grid := NewPosterGrid("Next Up")
			hs.convertItemsForGrid(grid, items)
			addResult(sectionResult{grid: grid, meta: sectionMeta{}, order: 1})
		}
	}()

	// Libraries — first get views, then load latest for each in parallel
	views, err := hs.client.GetViews()
	if err != nil {
		log.Printf("Failed to load views: %v", err)
		setError(err)
	} else {
		var libViews []struct{ ID, Name string }
		for _, view := range views {
			libViews = append(libViews, struct{ ID, Name string }{view.ID, view.Name})
		}
		hs.mu.Lock()
		hs.libraryViews = libViews
		hs.mu.Unlock()

		for i, view := range views {
			wg.Add(1)
			go func(view jellyfin.MediaItem, order int) {
				defer wg.Done()
				items, err := hs.client.GetLatestMedia(view.ID, 20)
				if err != nil {
					log.Printf("Failed to load latest for %s: %v", view.Name, err)
					return
				}
				if len(items) == 0 {
					return
				}
				grid := NewPosterGrid("Latest " + view.Name)
				hs.convertItemsForGrid(grid, items)
				grid.Items = append(grid.Items, GridItem{
					ID:    "_seeall_" + view.ID,
					Title: "See All >",
				})
				addResult(sectionResult{
					grid:  grid,
					meta:  sectionMeta{IsLibrary: true, ParentID: view.ID, Title: view.Name},
					order: order,
				})
			}(view, i+2) // order starts at 2 after Continue Watching and Next Up
		}
	}

	wg.Wait()

	// Sort results by order to maintain stable section ordering
	sort.Slice(results, func(i, j int) bool {
		return results[i].order < results[j].order
	})

	var sections []*PosterGrid
	var metas []sectionMeta
	for _, r := range results {
		sections = append(sections, r.grid)
		metas = append(metas, r.meta)
	}

	hs.mu.Lock()
	hs.sections = sections
	hs.sectionMeta = metas
	if len(sections) > 0 {
		sections[0].Active = true
	}
	if len(sections) == 0 && anyError != nil {
		errMsg := anyError.Error()
		hs.loadError = "Failed to load: " + errMsg
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "Unauthorized") {
			hs.authFailed = true
		}
	}
	hs.loaded = true
	hs.loading = false
	hs.mu.Unlock()
}

func (hs *HomeScreen) convertItemsForGrid(grid *PosterGrid, items []jellyfin.MediaItem) {
	result := make([]GridItem, len(items))
	for i, item := range items {
		result[i] = GridItemFromMediaItem(item)
	}
	grid.Items = result
	LoadGridItemImages(hs.client, hs.imgCache, &grid.Items, items, &hs.mu)
}

func (hs *HomeScreen) Update() (*ScreenTransition, error) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.authFailed && hs.OnAuthError != nil {
		hs.OnAuthError()
		return nil, nil
	}

	hs.ScrollState.HandleMouseWheel()

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && hs.errDisplay.HandleClick(mx, my, hs.loadError) {
		return nil, nil
	}
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
				if item != nil {
					if len(item.ID) > 8 && item.ID[:8] == "_seeall_" && hs.OnLibraryBrowse != nil {
						if i < len(hs.sectionMeta) && hs.sectionMeta[i].IsLibrary {
							meta := hs.sectionMeta[i]
							hs.OnLibraryBrowse(meta.ParentID, meta.Title)
						}
					} else if hs.OnItemSelected != nil {
						fullItem, err := hs.client.GetItem(item.ID)
						if err == nil {
							hs.OnItemSelected(*fullItem)
						}
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
				item.Watched = ToggleWatched(hs.client, item.ID, item.Watched)
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
		} else {
			// Focus the global navbar
			return &ScreenTransition{Type: TransitionFocusNavBar}, nil
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
		if item != nil {
			// Check if this is a "See All" pseudo-item
			if len(item.ID) > 8 && item.ID[:8] == "_seeall_" && hs.OnLibraryBrowse != nil {
				if hs.sectionIndex < len(hs.sectionMeta) && hs.sectionMeta[hs.sectionIndex].IsLibrary {
					meta := hs.sectionMeta[hs.sectionIndex]
					hs.OnLibraryBrowse(meta.ParentID, meta.Title)
				}
			} else if hs.OnItemSelected != nil {
				// Fetch full item data
				fullItem, err := hs.client.GetItem(item.ID)
				if err == nil {
					hs.OnItemSelected(*fullItem)
				}
			}
		}
	}

	return nil, nil
}

func (hs *HomeScreen) ensureSectionVisible() {
	sectionHeight := float64(SectionFullHeight)
	targetY := float64(hs.sectionIndex) * sectionHeight
	maxScroll := targetY - float64(ScreenHeight)/4
	if maxScroll < 0 {
		maxScroll = 0
	}
	hs.TargetScrollY = maxScroll
}

func (hs *HomeScreen) Draw(dst *ebiten.Image) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.ScrollState.Animate()

	if !hs.loaded {
		DrawTextCentered(dst, "Loading...", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	if hs.loadError != "" && len(hs.sections) == 0 {
		errX := float64(ScreenWidth)/2 - 300
		errY := float64(ScreenHeight)/2 - 20
		hs.errDisplay.Draw(dst, hs.loadError, errX, errY, FontSizeBody)
		DrawTextCentered(dst, "Press Enter to retry", float64(ScreenWidth)/2, float64(ScreenHeight)/2+20,
			FontSizeSmall, ColorTextMuted)
		return
	}

	if len(hs.sections) == 0 {
		DrawTextCentered(dst, "No media found", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	// Sections start below the navbar
	y := float64(NavBarHeight+10) - hs.ScrollY
	for _, section := range hs.sections {
		h := section.Draw(dst, SectionPadding, y)
		y += h + SectionGap
	}
}
