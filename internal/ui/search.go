package ui

import (
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
)

// SearchScreen provides text search with results grid.
type SearchScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache

	input       TextInput
	results     []jellyfin.MediaItem
	gridItems   []GridItem
	grid        *FocusGrid
	focusMode   int // 0=search bar, 1=results
	searchError string

	searching bool
	ScrollState

	OnItemSelected func(item jellyfin.MediaItem)

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

func NewSearchScreen(client *jellyfin.Client, imgCache *cache.ImageCache) *SearchScreen {
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)
	return &SearchScreen{
		client:   client,
		imgCache: imgCache,
		grid:     NewFocusGrid(cols, 0),
	}
}

func (ss *SearchScreen) Name() string { return "Search" }
func (ss *SearchScreen) OnEnter()     {}
func (ss *SearchScreen) OnExit()      {}

// SetInitialQuery sets the search text and triggers a search immediately.
func (ss *SearchScreen) SetInitialQuery(query string) {
	ss.input.SetText(query)
	go ss.doSearch()
}

func (ss *SearchScreen) Update() (*ScreenTransition, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	_, enter, back := InputState()

	if back {
		if ss.focusMode == 1 {
			ss.focusMode = 0
			return nil, nil
		}
		// If in search bar with query, Escape clears query first
		if ss.focusMode == 0 && ss.input.Text != "" {
			ss.input.Clear()
			ss.results = nil
			ss.gridItems = nil
			ss.grid.SetTotal(0)
			ss.searchError = ""
			return nil, nil
		}
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	ss.ScrollState.HandleMouseWheel()

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && ss.errDisplay.HandleClick(mx, my, ss.searchError) {
		return nil, nil
	}
	if clicked {
		// Check search bar click
		barX := float64(SectionPadding)
		barY := float64(NavBarHeight) + 20.0
		barW := float64(ScreenWidth - SectionPadding*2)
		barH := 44.0
		if PointInRect(mx, my, barX, barY, barW, barH) {
			// Check clear button click (right edge of search bar)
			if ss.input.Text != "" && PointInRect(mx, my, barX+barW-40, barY, 40, barH) {
				ss.input.Clear()
				ss.results = nil
				ss.gridItems = nil
				ss.grid.SetTotal(0)
				ss.searchError = ""
			}
			ss.focusMode = 0
			return nil, nil
		}
		// Check result items click
		if len(ss.gridItems) > 0 {
			resultBaseY := barY + barH + 40 - ss.ScrollY // 40 = gap + result count
			if idx, ok := ss.grid.HandleClick(mx, my, SectionPadding, resultBaseY); ok {
				ss.focusMode = 1
				ss.grid.Focused = idx
				if idx < len(ss.results) && ss.OnItemSelected != nil {
					ss.OnItemSelected(ss.results[idx])
				}
				return nil, nil
			}
		}
	}

	// Right-click: toggle watched state
	rmx, rmy, rclicked := MouseJustRightClicked()
	if rclicked && len(ss.gridItems) > 0 {
		barY := float64(NavBarHeight) + 20.0
		barH := 44.0
		resultBaseY := barY + barH + 40 - ss.ScrollY
		if idx, ok := ss.grid.HandleClick(rmx, rmy, SectionPadding, resultBaseY); ok {
			if idx < len(ss.results) {
				if ss.results[idx].Played {
					go ss.client.MarkUnplayed(ss.results[idx].ID)
				} else {
					go ss.client.MarkPlayed(ss.results[idx].ID)
				}
				ss.results[idx].Played = !ss.results[idx].Played
				ss.gridItems[idx].Watched = ss.results[idx].Played
			}
			return nil, nil
		}
	}

	switch ss.focusMode {
	case 0: // search bar
		ss.input.Update()

		if enter && ss.input.Text != "" {
			go ss.doSearch()
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			return &ScreenTransition{Type: TransitionFocusNavBar}, nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && len(ss.results) > 0 {
			ss.focusMode = 1
		}

	case 1: // results grid
		dir, _, _ := InputState()
		if dir != DirNone {
			if dir == DirUp && ss.grid.FocusedRow() == 0 {
				ss.focusMode = 0
			} else {
				ss.grid.Update(dir)
			}
		}

		if enter {
			idx := ss.grid.Focused
			if idx < len(ss.results) && ss.OnItemSelected != nil {
				ss.OnItemSelected(ss.results[idx])
			}
		}
	}

	return nil, nil
}

func (ss *SearchScreen) doSearch() {
	ss.mu.Lock()
	ss.searching = true
	ss.searchError = ""
	query := ss.input.Text
	ss.mu.Unlock()

	items, err := ss.client.SearchItems(query, 40)

	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.searching = false

	if err != nil {
		ss.searchError = "Search failed: " + err.Error()
		return
	}

	ss.results = items
	ss.grid.SetTotal(len(items))
	ss.grid.Focused = 0
	ss.ScrollState.Reset()

	ss.gridItems = make([]GridItem, len(items))
	for i, item := range items {
		ss.gridItems[i] = GridItemFromMediaItem(item)
	}
	LoadGridItemImages(ss.client, ss.imgCache, &ss.gridItems, items, &ss.mu)
}

func (ss *SearchScreen) Draw(dst *ebiten.Image) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.ScrollState.Animate()

	// Search bar (below navbar)
	barX := float32(SectionPadding)
	barY := float32(NavBarHeight + 20)
	barW := float32(ScreenWidth - SectionPadding*2)
	barH := float32(44)

	bgColor := ColorSurface
	if ss.focusMode == 0 {
		bgColor = ColorSurfaceHover
	}
	vector.DrawFilledRect(dst, barX, barY, barW, barH, bgColor, false)
	if ss.focusMode == 0 {
		vector.StrokeRect(dst, barX, barY, barW, barH, 2, ColorFocusBorder, false)
	}

	var displayQuery string
	if ss.focusMode == 0 {
		displayQuery = ss.input.DisplayText()
	} else {
		displayQuery = ss.input.Text
	}
	if ss.input.Text == "" && ss.focusMode != 0 {
		DrawText(dst, "Search...", float64(barX+12), float64(barY+12), FontSizeBody, ColorTextMuted)
	} else if ss.input.Text == "" && ss.focusMode == 0 {
		DrawText(dst, "Search...", float64(barX+12), float64(barY+12), FontSizeBody, ColorTextMuted)
		DrawText(dst, displayQuery, float64(barX+12), float64(barY+12), FontSizeBody, ColorText)
	} else {
		DrawText(dst, displayQuery, float64(barX+12), float64(barY+12), FontSizeBody, ColorText)
	}

	// Clear button
	if ss.input.Text != "" {
		clearX := float64(barX+barW) - 32
		clearY := float64(barY) + 10
		drawXMark(dst, float32(clearX), float32(clearY)+float32(barH)/2-10, 5, ColorTextMuted)
	}

	if ss.searching {
		DrawText(dst, "Searching...", float64(barX+barW-120), float64(barY+12), FontSizeSmall, ColorTextSecondary)
	}

	// Result count / error below search bar
	y := float64(barY+barH) + 8
	if ss.searchError != "" {
		y += ss.errDisplay.Draw(dst, ss.searchError, float64(barX), y, FontSizeSmall)
	} else if len(ss.results) > 0 {
		countStr := fmt.Sprintf("%d results", len(ss.results))
		DrawText(dst, countStr, float64(barX), y, FontSizeSmall, ColorTextMuted)
		y += FontSizeSmall + 8
	}

	y += 8 // gap before results

	// Results
	if len(ss.gridItems) == 0 && !ss.searching {
		if ss.input.Text != "" && len(ss.results) == 0 && ss.searchError == "" {
			DrawTextCentered(dst, "No results found", float64(ScreenWidth)/2, y+100,
				FontSizeHeading, ColorTextSecondary)
		}
		return
	}

	for i, item := range ss.gridItems {
		x, iy := ss.grid.ItemRect(i, SectionPadding, y-ss.ScrollY)

		// Skip offscreen
		if iy+PosterHeight < 0 || iy > float64(ScreenHeight) {
			continue
		}

		isFocused := ss.focusMode == 1 && i == ss.grid.Focused
		drawPosterItem(dst, item, x, iy, isFocused)
	}
}
