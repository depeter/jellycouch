package ui

import (
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyseerr"
)

// JellyseerrSearchScreen lets users search Jellyseerr for media to request.
type JellyseerrSearchScreen struct {
	client   *jellyseerr.Client
	imgCache *cache.ImageCache

	input     TextInput
	results   []jellyseerr.SearchResult
	gridItems []GridItem
	grid      *FocusGrid
	focusMode int // 0=search bar, 1=results
	searchErr string
	searching bool

	ScrollState

	OnResultSelected func(result jellyseerr.SearchResult)

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

func NewJellyseerrSearchScreen(client *jellyseerr.Client, imgCache *cache.ImageCache) *JellyseerrSearchScreen {
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)
	return &JellyseerrSearchScreen{
		client:   client,
		imgCache: imgCache,
		grid:     NewFocusGrid(cols, 0),
	}
}

func (js *JellyseerrSearchScreen) Name() string { return "Search" }
func (js *JellyseerrSearchScreen) OnEnter()     {}
func (js *JellyseerrSearchScreen) OnExit()      {}

func (js *JellyseerrSearchScreen) Update() (*ScreenTransition, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	_, enter, back := InputState()

	if back {
		if js.focusMode == 1 {
			js.focusMode = 0
			return nil, nil
		}
		if js.focusMode == 0 && js.input.Text != "" {
			js.input.Clear()
			js.results = nil
			js.gridItems = nil
			js.grid.SetTotal(0)
			js.searchErr = ""
			return nil, nil
		}
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	js.ScrollState.HandleMouseWheel()

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && js.errDisplay.HandleClick(mx, my, js.searchErr) {
		return nil, nil
	}
	if clicked {
		barX := float64(SectionPadding)
		barY := float64(NavBarHeight) + 20.0
		barW := float64(ScreenWidth - SectionPadding*2)
		barH := 44.0
		if PointInRect(mx, my, barX, barY, barW, barH) {
			if js.input.Text != "" && PointInRect(mx, my, barX+barW-40, barY, 40, barH) {
				js.input.Clear()
				js.results = nil
				js.gridItems = nil
				js.grid.SetTotal(0)
				js.searchErr = ""
			}
			js.focusMode = 0
			return nil, nil
		}
		if len(js.gridItems) > 0 {
			resultBaseY := barY + barH + 40 - js.ScrollY
			if idx, ok := js.grid.HandleClick(mx, my, SectionPadding, resultBaseY); ok {
				js.focusMode = 1
				js.grid.Focused = idx
				if idx < len(js.results) && js.OnResultSelected != nil {
					js.OnResultSelected(js.results[idx])
				}
				return nil, nil
			}
		}
	}

	switch js.focusMode {
	case 0: // search bar
		js.input.Update()

		if enter && js.input.Text != "" {
			go js.doSearch()
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			return &ScreenTransition{Type: TransitionFocusNavBar}, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && len(js.results) > 0 {
			js.focusMode = 1
		}

	case 1: // results grid
		dir, _, _ := InputState()
		if dir != DirNone {
			if dir == DirUp && js.grid.FocusedRow() == 0 {
				js.focusMode = 0
			} else {
				js.grid.Update(dir)
			}
		}

		if enter {
			idx := js.grid.Focused
			if idx < len(js.results) && js.OnResultSelected != nil {
				js.OnResultSelected(js.results[idx])
			}
		}
	}

	return nil, nil
}

func (js *JellyseerrSearchScreen) doSearch() {
	js.mu.Lock()
	js.searching = true
	js.searchErr = ""
	query := js.input.Text
	js.mu.Unlock()

	resp, err := js.client.Search(query, 1)

	js.mu.Lock()
	defer js.mu.Unlock()
	js.searching = false

	if err != nil {
		js.searchErr = "Search failed: " + err.Error()
		return
	}

	// Filter out "person" results
	var filtered []jellyseerr.SearchResult
	for _, r := range resp.Results {
		if r.MediaType == "movie" || r.MediaType == "tv" {
			filtered = append(filtered, r)
		}
	}

	js.results = filtered
	js.grid.SetTotal(len(filtered))
	js.grid.Focused = 0
	js.ScrollState.Reset()

	js.gridItems = make([]GridItem, len(filtered))
	for i, result := range filtered {
		js.gridItems[i] = GridItem{
			ID:            fmt.Sprintf("%d", result.ID),
			Title:         result.DisplayTitle(),
			Subtitle:      result.Year(),
			Rating:        result.VoteAverage,
			RequestStatus: js.mediaStatus(result),
		}
		posterURL := result.PosterURL()
		if posterURL == "" {
			continue
		}
		if img := js.imgCache.Get(posterURL); img != nil {
			js.gridItems[i].Image = img
		} else {
			idx := i
			js.imgCache.LoadAsync(posterURL, func(img *ebiten.Image) {
				js.mu.Lock()
				defer js.mu.Unlock()
				if idx < len(js.gridItems) {
					js.gridItems[idx].Image = img
				}
			})
		}
	}
}

func (js *JellyseerrSearchScreen) mediaStatus(r jellyseerr.SearchResult) int {
	if r.MediaInfo == nil {
		return 0
	}
	return r.MediaInfo.Status
}

func (js *JellyseerrSearchScreen) Draw(dst *ebiten.Image) {
	js.mu.Lock()
	defer js.mu.Unlock()

	js.ScrollState.Animate()

	// Title (below navbar)
	DrawText(dst, "Search", SectionPadding, NavBarHeight+16, FontSizeTitle, ColorPrimary)

	// Search bar
	barX := float32(SectionPadding)
	barY := float32(NavBarHeight + 58)
	barW := float32(ScreenWidth - SectionPadding*2)
	barH := float32(44)

	bgColor := ColorSurface
	if js.focusMode == 0 {
		bgColor = ColorSurfaceHover
	}
	vector.DrawFilledRect(dst, barX, barY, barW, barH, bgColor, false)
	if js.focusMode == 0 {
		vector.StrokeRect(dst, barX, barY, barW, barH, 2, ColorFocusBorder, false)
	}

	var displayQuery string
	if js.focusMode == 0 {
		displayQuery = js.input.DisplayText()
	} else {
		displayQuery = js.input.Text
	}
	if js.input.Text == "" {
		DrawText(dst, "Search movies & TV shows...", float64(barX+12), float64(barY+12), FontSizeBody, ColorTextMuted)
	}
	if displayQuery != "" {
		DrawText(dst, displayQuery, float64(barX+12), float64(barY+12), FontSizeBody, ColorText)
	}

	// Clear button
	if js.input.Text != "" {
		clearX := float64(barX+barW) - 32
		DrawTextCentered(dst, "\u2715", clearX, float64(barY)+float64(barH)/2, FontSizeBody, ColorTextMuted)
	}

	if js.searching {
		DrawText(dst, "Searching...", float64(barX+barW-120), float64(barY+12), FontSizeSmall, ColorTextSecondary)
	}

	// Result count / error
	y := float64(barY+barH) + 8
	if js.searchErr != "" {
		y += js.errDisplay.Draw(dst, js.searchErr, float64(barX), y, FontSizeSmall)
	} else if len(js.results) > 0 {
		DrawText(dst, fmt.Sprintf("%d results", len(js.results)), float64(barX), y, FontSizeSmall, ColorTextMuted)
		y += FontSizeSmall + 8
	}
	y += 8

	if len(js.gridItems) == 0 && !js.searching {
		if js.input.Text != "" && len(js.results) == 0 && js.searchErr == "" {
			DrawTextCentered(dst, "No results found", float64(ScreenWidth)/2, y+100,
				FontSizeHeading, ColorTextSecondary)
		}
		return
	}

	for i, item := range js.gridItems {
		x, iy := js.grid.ItemRect(i, SectionPadding, y-js.ScrollY)

		if iy+PosterHeight < 0 || iy > float64(ScreenHeight) {
			continue
		}

		isFocused := js.focusMode == 1 && i == js.grid.Focused
		drawPosterItem(dst, item, x, iy, isFocused)
	}
}
