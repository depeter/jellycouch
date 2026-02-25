package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyseerr"
)

// JellyseerrDiscoverScreen shows trending and popular content from Jellyseerr.
type JellyseerrDiscoverScreen struct {
	client   *jellyseerr.Client
	imgCache *cache.ImageCache

	sections     []*PosterGrid
	results      [][]jellyseerr.SearchResult // parallel to sections
	sectionIndex int
	focusMode    int // 0=header buttons, 1=sections
	loaded       bool
	loading      bool
	loadError    string
	scrollY      float64
	targetScrollY float64

	OnItemSelected func(result jellyseerr.SearchResult)
	OnRequests     func()
	OnSearch       func()

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

func NewJellyseerrDiscoverScreen(client *jellyseerr.Client, imgCache *cache.ImageCache) *JellyseerrDiscoverScreen {
	return &JellyseerrDiscoverScreen{
		client:    client,
		imgCache:  imgCache,
		focusMode: 1,
	}
}

func (ds *JellyseerrDiscoverScreen) Name() string { return "Discover" }

func (ds *JellyseerrDiscoverScreen) OnEnter() {
	if !ds.loaded && !ds.loading {
		ds.loading = true
		go ds.loadData()
	}
}

func (ds *JellyseerrDiscoverScreen) OnExit() {}

func (ds *JellyseerrDiscoverScreen) loadData() {
	type sectionData struct {
		label   string
		results []jellyseerr.SearchResult
	}

	type fetchResult struct {
		index int
		data  sectionData
		err   error
	}

	fetchers := []struct {
		label string
		fetch func() (*jellyseerr.SearchResponse, error)
	}{
		{"Trending", func() (*jellyseerr.SearchResponse, error) { return ds.client.GetTrending(1) }},
		{"Popular Movies", func() (*jellyseerr.SearchResponse, error) { return ds.client.GetDiscoverMovies(1) }},
		{"Popular TV Shows", func() (*jellyseerr.SearchResponse, error) { return ds.client.GetDiscoverTV(1) }},
	}

	ch := make(chan fetchResult, len(fetchers))
	for i, f := range fetchers {
		i, f := i, f
		go func() {
			resp, err := f.fetch()
			if err != nil {
				ch <- fetchResult{index: i, err: err}
				return
			}
			// Filter out "person" results
			var filtered []jellyseerr.SearchResult
			for _, r := range resp.Results {
				if r.MediaType == "movie" || r.MediaType == "tv" {
					filtered = append(filtered, r)
				}
			}
			ch <- fetchResult{index: i, data: sectionData{label: f.label, results: filtered}}
		}()
	}

	ordered := make([]fetchResult, len(fetchers))
	var anyError error
	for range fetchers {
		r := <-ch
		ordered[r.index] = r
		if r.err != nil {
			anyError = r.err
		}
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.loading = false
	ds.loaded = true

	var sections []*PosterGrid
	var allResults [][]jellyseerr.SearchResult

	for _, r := range ordered {
		if r.err != nil {
			log.Printf("Discover: failed to load %s: %v", fetchers[r.index].label, r.err)
			continue
		}
		if len(r.data.results) == 0 {
			continue
		}

		grid := NewPosterGrid(r.data.label)
		items := make([]GridItem, len(r.data.results))
		for i, result := range r.data.results {
			items[i] = GridItem{
				ID:            fmt.Sprintf("%d", result.ID),
				Title:         result.DisplayTitle(),
				Subtitle:      result.Year(),
				RequestStatus: ds.mediaStatus(result),
			}
			posterURL := result.PosterURL()
			if posterURL == "" {
				continue
			}
			if img := ds.imgCache.Get(posterURL); img != nil {
				items[i].Image = img
			} else {
				idx := i
				ds.imgCache.LoadAsync(posterURL, func(img *ebiten.Image) {
					ds.mu.Lock()
					defer ds.mu.Unlock()
					if idx < len(grid.Items) {
						grid.Items[idx].Image = img
					}
				})
			}
		}
		grid.Items = items
		sections = append(sections, grid)
		allResults = append(allResults, r.data.results)
	}

	ds.sections = sections
	ds.results = allResults
	if len(sections) > 0 {
		sections[0].Active = true
	}
	if len(sections) == 0 && anyError != nil {
		ds.loadError = "Failed to load: " + anyError.Error()
	}
}

func (ds *JellyseerrDiscoverScreen) mediaStatus(r jellyseerr.SearchResult) int {
	if r.MediaInfo == nil {
		return 0
	}
	return r.MediaInfo.Status
}

func (ds *JellyseerrDiscoverScreen) Update() (*ScreenTransition, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	_, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Mouse wheel scroll
	_, wy := MouseWheelDelta()
	if wy != 0 {
		ds.targetScrollY -= wy * 60
		if ds.targetScrollY < 0 {
			ds.targetScrollY = 0
		}
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && ds.errDisplay.HandleClick(mx, my, ds.loadError) {
		return nil, nil
	}
	if clicked {
		// Requests button
		reqX := float64(ScreenWidth) - SectionPadding - 230.0
		reqY := 14.0
		if PointInRect(mx, my, reqX, reqY, 100, 34) {
			if ds.OnRequests != nil {
				ds.OnRequests()
			}
			return nil, nil
		}
		// Search button
		searchX := float64(ScreenWidth) - SectionPadding - 120.0
		searchY := 14.0
		if PointInRect(mx, my, searchX, searchY, 120, 34) {
			if ds.OnSearch != nil {
				ds.OnSearch()
			}
			return nil, nil
		}
	}
	if clicked && ds.loaded && len(ds.sections) > 0 {
		for i, section := range ds.sections {
			if idx, ok := section.HandleClick(mx, my); ok {
				if ds.focusMode == 1 {
					ds.sections[ds.sectionIndex].Active = false
				}
				ds.focusMode = 1
				ds.sectionIndex = i
				ds.sections[ds.sectionIndex].Active = true
				section.Focused = idx

				if i < len(ds.results) && idx < len(ds.results[i]) && ds.OnItemSelected != nil {
					ds.OnItemSelected(ds.results[i][idx])
				}
				return nil, nil
			}
		}
	}

	// Keyboard shortcuts
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		if ds.OnRequests != nil {
			ds.OnRequests()
		}
		return nil, nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		if ds.OnSearch != nil {
			ds.OnSearch()
		}
		return nil, nil
	}

	if !ds.loaded || len(ds.sections) == 0 {
		return nil, nil
	}

	dir, _, _ := InputState()
	currentSection := ds.sections[ds.sectionIndex]

	switch dir {
	case DirUp:
		if ds.sectionIndex > 0 {
			currentSection.Active = false
			ds.sectionIndex--
			ds.sections[ds.sectionIndex].Active = true
			ds.ensureSectionVisible()
		}
	case DirDown:
		if ds.sectionIndex < len(ds.sections)-1 {
			currentSection.Active = false
			ds.sectionIndex++
			ds.sections[ds.sectionIndex].Active = true
			ds.ensureSectionVisible()
		}
	case DirLeft, DirRight:
		currentSection.Update(dir)
	}

	if enter {
		item := currentSection.SelectedItem()
		if item != nil && ds.OnItemSelected != nil {
			idx := currentSection.Focused
			if ds.sectionIndex < len(ds.results) && idx < len(ds.results[ds.sectionIndex]) {
				ds.OnItemSelected(ds.results[ds.sectionIndex][idx])
			}
		}
	}

	return nil, nil
}

func (ds *JellyseerrDiscoverScreen) ensureSectionVisible() {
	sectionHeight := float64(PosterHeight + FontSizeSmall + 16 + PosterFocusPad*2 + SectionTitleH + SectionGap)
	targetY := float64(ds.sectionIndex) * sectionHeight
	maxScroll := targetY - float64(ScreenHeight)/4
	if maxScroll < 0 {
		maxScroll = 0
	}
	ds.targetScrollY = maxScroll
}

func (ds *JellyseerrDiscoverScreen) Draw(dst *ebiten.Image) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.scrollY = Lerp(ds.scrollY, ds.targetScrollY, ScrollAnimSpeed)

	// Header
	DrawText(dst, "Discover", SectionPadding, 16, FontSizeTitle, ColorPrimary)

	// Requests button
	reqX := float64(ScreenWidth) - SectionPadding - 230.0
	reqY := 14.0
	vector.DrawFilledRect(dst, float32(reqX), float32(reqY), 100, 34, ColorSurface, false)
	DrawTextCentered(dst, "Requests", reqX+50, reqY+17, FontSizeSmall, ColorTextSecondary)

	// Search button
	searchX := float64(ScreenWidth) - SectionPadding - 120.0
	searchY := 14.0
	vector.DrawFilledRect(dst, float32(searchX), float32(searchY), 120, 34, ColorSurface, false)
	DrawTextCentered(dst, "\U0001F50D Search", searchX+60, searchY+17, FontSizeSmall, ColorTextSecondary)

	if !ds.loaded {
		DrawTextCentered(dst, "Loading...", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	if ds.loadError != "" && len(ds.sections) == 0 {
		errX := float64(ScreenWidth)/2 - 300
		errY := float64(ScreenHeight)/2 - 20
		ds.errDisplay.Draw(dst, ds.loadError, errX, errY, FontSizeBody)
		return
	}

	if len(ds.sections) == 0 {
		DrawTextCentered(dst, "No content found", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	y := float64(NavBarHeight+10) - ds.scrollY
	for _, section := range ds.sections {
		h := section.Draw(dst, SectionPadding, y)
		y += h + SectionGap
	}

	// Hint
	hint := "R requests  \u00b7  / search  \u00b7  Esc back"
	DrawText(dst, hint, SectionPadding, float64(ScreenHeight)-40, FontSizeSmall, ColorTextMuted)
}
