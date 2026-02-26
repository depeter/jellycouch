package ui

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

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
	focusMode    int // 0=search bar, 1=sections, 2=nav buttons, 3=library nav
	navBtnIndex  int // 0=discovery, 1=settings (when focusMode==2)
	libNavIndex  int // which library button is focused (when focusMode==3)
	input        TextInput
	loaded       bool
	loading      bool
	loadError    string
	scrollY      float64
	targetScrollY float64

	// Library views for nav buttons
	libraryViews []struct{ ID, Name string }

	// Per-section metadata for library browsing
	sectionMeta []sectionMeta

	// Callbacks
	OnItemSelected    func(item jellyfin.MediaItem)
	OnSearch          func(query string)
	OnSettings        func()
	OnRequests        func()
	OnAuthError       func()
	OnLibraryBrowse   func(parentID, title string)
	JellyseerrEnabled func() bool

	authFailed bool
	errDisplay ErrorDisplay

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
	if hs.libNavIndex >= len(hs.libraryViews) {
		hs.libNavIndex = 0
	}
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
		result[i] = GridItem{
			ID:    item.ID,
			Title: item.Name,
		}
		// For episodes, show the series name as the title and episode info as subtitle
		if item.Type == "Episode" && item.SeriesName != "" {
			result[i].Title = item.SeriesName
			ep := fmt.Sprintf("S%dE%d", item.ParentIndexNumber, item.IndexNumber)
			if item.Name != "" {
				ep += " · " + item.Name
			}
			result[i].Subtitle = ep
		} else if item.Year > 0 {
			result[i].Subtitle = fmt.Sprintf("%d", item.Year)
		}

		// Flow progress and watched state
		if item.RuntimeTicks > 0 && item.PlaybackPositionTicks > 0 {
			result[i].Progress = float64(item.PlaybackPositionTicks) / float64(item.RuntimeTicks)
		}
		result[i].Watched = item.Played
		result[i].Rating = float64(item.CommunityRating)

		// For episodes, show series poster instead of episode thumbnail
		posterID := item.ID
		if item.Type == "Episode" && item.SeriesID != "" {
			posterID = item.SeriesID
		}

		// Async load poster image — capture grid pointer and item ID for race-safe callback
		url := hs.client.GetPosterURL(posterID)
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

	if hs.authFailed && hs.OnAuthError != nil {
		hs.OnAuthError()
		return nil, nil
	}

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
	if clicked && hs.errDisplay.HandleClick(mx, my, hs.loadError) {
		return nil, nil
	}
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
			hs.navBtnIndex = 0
			return nil, nil
		}
		// Settings button click
		settingsX := float64(ScreenWidth) - SectionPadding - 100
		settingsY := 12.0
		settingsW := 100.0
		settingsH := 38.0
		if PointInRect(mx, my, settingsX, settingsY, settingsW, settingsH) {
			if hs.OnSettings != nil {
				hs.OnSettings()
			}
			return nil, nil
		}
		// Discovery button click (only when Jellyseerr is configured)
		if hs.JellyseerrEnabled != nil && hs.JellyseerrEnabled() {
			reqX := settingsX - 120
			reqY := 12.0
			reqW := 110.0
			reqH := 38.0
			if PointInRect(mx, my, reqX, reqY, reqW, reqH) {
				if hs.OnRequests != nil {
					hs.OnRequests()
				}
				return nil, nil
			}
		}

		// Library nav button clicks
		libBtnX := 230.0
		for _, view := range hs.libraryViews {
			tw, _ := MeasureText(view.Name, FontSizeBody)
			btnW := tw + 28
			if PointInRect(mx, my, libBtnX, 12, btnW, 38) {
				if hs.focusMode == 1 && len(hs.sections) > 0 {
					hs.sections[hs.sectionIndex].Active = false
				}
				if hs.OnLibraryBrowse != nil {
					hs.OnLibraryBrowse(view.ID, view.Name)
				}
				return nil, nil
			}
			libBtnX += btnW + 10
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
			return nil, nil
		}

		// Left arrow at start of input → move focus to library nav
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) && hs.input.CursorAtStart() && len(hs.libraryViews) > 0 {
			hs.libNavIndex = len(hs.libraryViews) - 1
			hs.focusMode = 3
			return nil, nil
		}

		// Right arrow at end of input text → move focus to nav buttons
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) && hs.input.CursorAtEnd() {
			if hs.JellyseerrEnabled != nil && hs.JellyseerrEnabled() {
				hs.navBtnIndex = 0 // Discovery
			} else {
				hs.navBtnIndex = 1 // Settings
			}
			hs.focusMode = 2
			return nil, nil
		}

		return nil, nil
	}

	// Nav buttons focused (right side: Discovery/Settings)
	if hs.focusMode == 2 {
		hasDiscovery := hs.JellyseerrEnabled != nil && hs.JellyseerrEnabled()

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if hs.navBtnIndex == 0 && hasDiscovery {
				if hs.OnRequests != nil {
					hs.OnRequests()
				}
			} else {
				if hs.OnSettings != nil {
					hs.OnSettings()
				}
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

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if hs.navBtnIndex == 0 && hasDiscovery {
				hs.navBtnIndex = 1 // Settings
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if hs.navBtnIndex == 1 && hasDiscovery {
				hs.navBtnIndex = 0 // Discovery
			} else {
				hs.focusMode = 0 // Back to search bar
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && hs.loaded && len(hs.sections) > 0 {
			hs.focusMode = 1
			hs.sections[hs.sectionIndex].Active = true
			return nil, nil
		}

		return nil, nil
	}

	// Library nav buttons focused (left side: Movies, TV Shows, etc.)
	if hs.focusMode == 3 && len(hs.libraryViews) > 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			view := hs.libraryViews[hs.libNavIndex]
			if hs.OnLibraryBrowse != nil {
				hs.OnLibraryBrowse(view.ID, view.Name)
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

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if hs.libNavIndex < len(hs.libraryViews)-1 {
				hs.libNavIndex++
			} else {
				hs.focusMode = 0 // Move to search bar
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if hs.libNavIndex > 0 {
				hs.libNavIndex--
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && hs.loaded && len(hs.sections) > 0 {
			hs.focusMode = 1
			hs.sections[hs.sectionIndex].Active = true
			return nil, nil
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

	if inpututil.IsKeyJustPressed(ebiten.KeyR) && hs.JellyseerrEnabled != nil && hs.JellyseerrEnabled() {
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
	sectionHeight := float64(PosterHeight + FontSizeSmall + FontSizeCaption + 24 + PosterFocusPad*2 + SectionTitleH + SectionGap)
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

	// Header
	DrawText(dst, "JellyCouch", SectionPadding, 16, FontSizeTitle, ColorPrimary)

	// Library nav buttons (between title and search bar)
	libBtnX := 230.0
	for i, view := range hs.libraryViews {
		tw, _ := MeasureText(view.Name, FontSizeBody)
		btnW := tw + 28
		btnH := 38.0
		btnY := 12.0
		focused := hs.focusMode == 3 && i == hs.libNavIndex
		if focused {
			vector.DrawFilledRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), ColorPrimary, false)
			DrawTextCentered(dst, view.Name, libBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorBackground)
		} else {
			vector.DrawFilledRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(libBtnX), float32(btnY), float32(btnW), float32(btnH), 1, ColorPrimary, false)
			DrawTextCentered(dst, view.Name, libBtnX+btnW/2, btnY+btnH/2, FontSizeBody, ColorText)
		}
		libBtnX += btnW + 10
	}

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
			DrawText(dst, "Search library...", searchX+14, searchY+10, FontSizeBody, ColorTextMuted)
		}
	}

	// Discovery button (only when Jellyseerr configured)
	settingsX := float64(ScreenWidth) - SectionPadding - 100
	if hs.JellyseerrEnabled != nil && hs.JellyseerrEnabled() {
		reqX := settingsX - 120
		reqY := 12.0
		reqW := 110.0
		reqH := 38.0
		focused := hs.focusMode == 2 && hs.navBtnIndex == 0
		if focused {
			vector.DrawFilledRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), ColorPrimary, false)
			DrawTextCentered(dst, "Discovery", reqX+reqW/2+8, reqY+reqH/2, FontSizeBody, ColorBackground)
			drawCompassIcon(dst, float32(reqX+16), float32(reqY+reqH/2), 7, ColorBackground)
		} else {
			vector.DrawFilledRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), ColorSurfaceHover, false)
			vector.StrokeRect(dst, float32(reqX), float32(reqY), float32(reqW), float32(reqH), 1, ColorPrimary, false)
			DrawTextCentered(dst, "Discovery", reqX+reqW/2+8, reqY+reqH/2, FontSizeBody, ColorText)
			drawCompassIcon(dst, float32(reqX+16), float32(reqY+reqH/2), 7, ColorPrimary)
		}
	}

	// Settings button
	settingsY := 12.0
	settingsW := 100.0
	settingsH := 38.0
	sfocused := hs.focusMode == 2 && hs.navBtnIndex == 1
	if sfocused {
		vector.DrawFilledRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), ColorPrimary, false)
		DrawTextCentered(dst, "Settings", settingsX+settingsW/2+8, settingsY+settingsH/2, FontSizeBody, ColorBackground)
		drawGearIcon(dst, float32(settingsX+16), float32(settingsY+settingsH/2), 7, ColorBackground)
	} else {
		vector.DrawFilledRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), ColorSurfaceHover, false)
		vector.StrokeRect(dst, float32(settingsX), float32(settingsY), float32(settingsW), float32(settingsH), 1, ColorTextSecondary, false)
		DrawTextCentered(dst, "Settings", settingsX+settingsW/2+8, settingsY+settingsH/2, FontSizeBody, ColorText)
		drawGearIcon(dst, float32(settingsX+16), float32(settingsY+settingsH/2), 7, ColorTextSecondary)
	}

	y := float64(NavBarHeight+10) - hs.scrollY
	for _, section := range hs.sections {
		h := section.Draw(dst, SectionPadding, y)
		y += h + SectionGap
	}
}

