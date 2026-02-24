package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
)

// LibraryScreen shows a grid of all items in a library.
type LibraryScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache

	parentID  string
	title     string
	itemTypes []string

	items []jellyfin.MediaItem
	grid  *FocusGrid
	gridItems []GridItem
	total int

	loaded  bool
	loading bool
	scrollY float64
	targetScrollY float64

	OnItemSelected func(item jellyfin.MediaItem)

	mu sync.Mutex
}

func NewLibraryScreen(client *jellyfin.Client, imgCache *cache.ImageCache, parentID, title string, itemTypes []string) *LibraryScreen {
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)
	return &LibraryScreen{
		client:    client,
		imgCache:  imgCache,
		parentID:  parentID,
		title:     title,
		itemTypes: itemTypes,
		grid:      NewFocusGrid(cols, 0),
	}
}

func (ls *LibraryScreen) Name() string { return "Library: " + ls.title }

func (ls *LibraryScreen) OnEnter() {
	if !ls.loaded && !ls.loading {
		ls.loading = true
		go ls.loadData(0)
	}
}

func (ls *LibraryScreen) OnExit() {}

func (ls *LibraryScreen) loadData(start int) {
	items, total, err := ls.client.GetItems(ls.parentID, start, 50, ls.itemTypes)
	if err != nil {
		log.Printf("Failed to load library items: %v", err)
		ls.mu.Lock()
		ls.loading = false
		ls.mu.Unlock()
		return
	}

	ls.mu.Lock()
	ls.items = append(ls.items, items...)
	ls.total = total
	ls.grid.SetTotal(len(ls.items))

	// Build grid items
	ls.gridItems = make([]GridItem, len(ls.items))
	for i, item := range ls.items {
		ls.gridItems[i] = GridItem{
			ID:    item.ID,
			Title: item.Name,
		}
		if item.Year > 0 {
			ls.gridItems[i].Subtitle = fmt.Sprintf("%d", item.Year)
		}

		url := ls.client.GetPosterURL(item.ID)
		if img := ls.imgCache.Get(url); img != nil {
			ls.gridItems[i].Image = img
		} else {
			idx := i
			ls.imgCache.LoadAsync(url, func(img *ebiten.Image) {
				ls.mu.Lock()
				defer ls.mu.Unlock()
				if idx < len(ls.gridItems) {
					ls.gridItems[idx].Image = img
				}
			})
		}
	}

	ls.loaded = true
	ls.loading = false
	ls.mu.Unlock()
}

func (ls *LibraryScreen) Update() (*ScreenTransition, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	dir, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	if !ls.loaded {
		return nil, nil
	}

	if dir != DirNone {
		ls.grid.Update(dir)
		ls.ensureVisible()
	}

	if enter {
		idx := ls.grid.Focused
		if idx < len(ls.items) && ls.OnItemSelected != nil {
			ls.OnItemSelected(ls.items[idx])
		}
	}

	return nil, nil
}

func (ls *LibraryScreen) ensureVisible() {
	row := ls.grid.FocusedRow()
	rowH := float64(PosterHeight + PosterGap + FontSizeCaption + 8)
	targetY := float64(row)*rowH - float64(ScreenHeight)/3
	if targetY < 0 {
		targetY = 0
	}
	ls.targetScrollY = targetY
}

func (ls *LibraryScreen) Draw(dst *ebiten.Image) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.scrollY = Lerp(ls.scrollY, ls.targetScrollY, ScrollAnimSpeed)

	// Title bar
	DrawText(dst, ls.title, SectionPadding, 16, FontSizeTitle, ColorText)
	if ls.total > 0 {
		countStr := fmt.Sprintf("%d items", ls.total)
		DrawText(dst, countStr, float64(ScreenWidth)-200, 24, FontSizeSmall, ColorTextMuted)
	}

	if !ls.loaded {
		DrawTextCentered(dst, "Loading...", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	// Draw grid
	baseY := float64(NavBarHeight+10) - ls.scrollY
	for i, item := range ls.gridItems {
		col := i % ls.grid.Cols
		row := i / ls.grid.Cols

		x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
		y := baseY + float64(row)*(PosterHeight+PosterGap+FontSizeCaption+8)

		// Skip offscreen
		if y+PosterHeight < 0 || y > float64(ScreenHeight) {
			continue
		}

		isFocused := i == ls.grid.Focused
		drawPosterItem(dst, item, x, y, isFocused)
	}
}

func drawPosterItem(dst *ebiten.Image, item GridItem, x, y float64, focused bool) {
	if focused {
		DrawFilledRoundRect(dst,
			float32(x-PosterFocusPad), float32(y-PosterFocusPad),
			float32(PosterWidth+PosterFocusPad*2), float32(PosterHeight+PosterFocusPad*2),
			4, ColorFocusBorder)
	}

	if item.Image != nil {
		op := &ebiten.DrawImageOptions{}
		bounds := item.Image.Bounds()
		scaleX := float64(PosterWidth) / float64(bounds.Dx())
		scaleY := float64(PosterHeight) / float64(bounds.Dy())
		op.GeoM.Scale(scaleX, scaleY)
		op.GeoM.Translate(x, y)
		dst.DrawImage(item.Image, op)
	} else {
		DrawFilledRoundRect(dst, float32(x), float32(y),
			float32(PosterWidth), float32(PosterHeight), 4, ColorSurface)
		DrawTextCentered(dst, item.Title, x+PosterWidth/2, y+PosterHeight/2,
			FontSizeSmall, ColorTextMuted)
	}

	titleColor := ColorTextSecondary
	if focused {
		titleColor = ColorText
	}
	title := truncateText(item.Title, PosterWidth, FontSizeCaption)
	DrawText(dst, title, x, y+PosterHeight+4, FontSizeCaption, titleColor)
}
