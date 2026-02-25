package ui

import (
	"fmt"
	"image/color"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

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
	focusMode    int // 0=nav buttons, 1=sections
	navBtnIndex  int // 0=My Requests, 1=Search (when focusMode==0)
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

func (ds *JellyseerrDiscoverScreen) Name() string { return "Discovery" }

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
				Rating:        result.VoteAverage,
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

// Nav button layout constants
const (
	discNavBtnY  = 12.0
	discNavBtnH  = 38.0
	discReqBtnW  = 130.0
	discSrchBtnW = 100.0
	discBtnGap   = 10.0
)

func (ds *JellyseerrDiscoverScreen) searchBtnX() float64 {
	return float64(ScreenWidth) - SectionPadding - discSrchBtnW
}
func (ds *JellyseerrDiscoverScreen) reqBtnX() float64 {
	return ds.searchBtnX() - discBtnGap - discReqBtnW
}

func (ds *JellyseerrDiscoverScreen) Update() (*ScreenTransition, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	_, enter, back := InputState()

	if back {
		if ds.focusMode == 0 {
			// From nav buttons, go to sections
			ds.focusMode = 1
			if len(ds.sections) > 0 {
				ds.sections[ds.sectionIndex].Active = true
			}
			return nil, nil
		}
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
		// My Requests button
		reqX := ds.reqBtnX()
		if PointInRect(mx, my, reqX, discNavBtnY, discReqBtnW, discNavBtnH) {
			if ds.OnRequests != nil {
				ds.OnRequests()
			}
			return nil, nil
		}
		// Search button
		searchX := ds.searchBtnX()
		if PointInRect(mx, my, searchX, discNavBtnY, discSrchBtnW, discNavBtnH) {
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

	// Nav buttons focused
	if ds.focusMode == 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || enter {
			if ds.navBtnIndex == 0 {
				if ds.OnRequests != nil {
					ds.OnRequests()
				}
			} else {
				if ds.OnSearch != nil {
					ds.OnSearch()
				}
			}
			return nil, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if ds.navBtnIndex == 0 {
				ds.navBtnIndex = 1
			}
			return nil, nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if ds.navBtnIndex == 1 {
				ds.navBtnIndex = 0
			}
			return nil, nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && ds.loaded && len(ds.sections) > 0 {
			ds.focusMode = 1
			ds.sections[ds.sectionIndex].Active = true
			return nil, nil
		}
		return nil, nil
	}

	// Keyboard shortcuts (from sections)
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
		} else {
			// Move focus to nav buttons
			currentSection.Active = false
			ds.focusMode = 0
			ds.navBtnIndex = 0
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
	DrawText(dst, "Discovery", SectionPadding, 16, FontSizeTitle, ColorPrimary)

	// My Requests button
	reqX := float32(ds.reqBtnX())
	drawNavButton(dst, "My Requests", reqX, discNavBtnY, discReqBtnW, discNavBtnH,
		ds.focusMode == 0 && ds.navBtnIndex == 0,
		func(d *ebiten.Image, cx, cy, r float32, c color.Color) { drawListIcon(d, cx, cy, r, c) },
		ColorAccent)

	// Search button
	searchX := float32(ds.searchBtnX())
	drawNavButton(dst, "Search", searchX, discNavBtnY, discSrchBtnW, discNavBtnH,
		ds.focusMode == 0 && ds.navBtnIndex == 1,
		func(d *ebiten.Image, cx, cy, r float32, c color.Color) { drawSearchIcon(d, cx, cy, r, c) },
		ColorPrimary)

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
