package ui

import (
	"sync"
	"unicode/utf8"

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

	query     string
	results   []jellyfin.MediaItem
	gridItems []GridItem
	grid      *FocusGrid
	focusMode int // 0=search bar, 1=results

	searching bool

	OnItemSelected func(item jellyfin.MediaItem)

	mu sync.Mutex
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

func (ss *SearchScreen) Update() (*ScreenTransition, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	_, enter, back := InputState()

	if back {
		if ss.focusMode == 1 {
			ss.focusMode = 0
			return nil, nil
		}
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	switch ss.focusMode {
	case 0: // search bar
		// Text input
		runes := ebiten.AppendInputChars(nil)
		if len(runes) > 0 {
			ss.query += string(runes)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(ss.query) > 0 {
			_, size := utf8.DecodeLastRuneInString(ss.query)
			ss.query = ss.query[:len(ss.query)-size]
		}

		if enter && ss.query != "" {
			go ss.doSearch()
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
	query := ss.query
	ss.mu.Unlock()

	items, err := ss.client.SearchItems(query, 40)

	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.searching = false

	if err != nil {
		return
	}

	ss.results = items
	ss.grid.SetTotal(len(items))
	ss.grid.Focused = 0

	ss.gridItems = make([]GridItem, len(items))
	for i, item := range items {
		ss.gridItems[i] = GridItem{
			ID:    item.ID,
			Title: item.Name,
		}
		url := ss.client.GetPosterURL(item.ID)
		if img := ss.imgCache.Get(url); img != nil {
			ss.gridItems[i].Image = img
		} else {
			idx := i
			ss.imgCache.LoadAsync(url, func(img *ebiten.Image) {
				ss.mu.Lock()
				defer ss.mu.Unlock()
				if idx < len(ss.gridItems) {
					ss.gridItems[idx].Image = img
				}
			})
		}
	}
}

func (ss *SearchScreen) Draw(dst *ebiten.Image) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Search bar
	barX := float32(SectionPadding)
	barY := float32(20)
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

	displayQuery := ss.query
	if ss.focusMode == 0 {
		displayQuery += "│"
	}
	if displayQuery == "│" || displayQuery == "" {
		DrawText(dst, "Search...", float64(barX+12), float64(barY+12), FontSizeBody, ColorTextMuted)
	}
	DrawText(dst, displayQuery, float64(barX+12), float64(barY+12), FontSizeBody, ColorText)

	if ss.searching {
		DrawText(dst, "Searching...", float64(barX+barW-120), float64(barY+12), FontSizeSmall, ColorTextSecondary)
	}

	// Results
	y := float64(barY+barH) + 20
	if len(ss.gridItems) == 0 && !ss.searching {
		if ss.query != "" && len(ss.results) == 0 {
			DrawTextCentered(dst, "No results found", float64(ScreenWidth)/2, y+100,
				FontSizeHeading, ColorTextSecondary)
		}
		return
	}

	for i, item := range ss.gridItems {
		col := i % ss.grid.Cols
		row := i / ss.grid.Cols

		x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
		iy := y + float64(row)*(PosterHeight+PosterGap+FontSizeCaption+8)

		isFocused := ss.focusMode == 1 && i == ss.grid.Focused
		drawPosterItem(dst, item, x, iy, isFocused)
	}
}
