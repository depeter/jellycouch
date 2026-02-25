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

	loaded      bool
	loading     bool
	loadingMore bool
	loadError   string
	scrollY     float64
	targetScrollY float64

	OnItemSelected func(item jellyfin.MediaItem)

	errDisplay ErrorDisplay
	mu         sync.Mutex
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
		ls.loadingMore = false
		ls.loadError = "Failed to load: " + err.Error()
		ls.mu.Unlock()
		return
	}

	ls.mu.Lock()
	ls.items = append(ls.items, items...)
	ls.total = total
	ls.grid.SetTotal(len(ls.items))

	// Build grid items for all items (rebuild to keep indices consistent)
	ls.gridItems = make([]GridItem, len(ls.items))
	for i, item := range ls.items {
		ls.gridItems[i] = GridItem{
			ID:    item.ID,
			Title: item.Name,
		}
		if item.Year > 0 {
			ls.gridItems[i].Subtitle = fmt.Sprintf("%d", item.Year)
		}

		// Flow progress and watched state
		if item.RuntimeTicks > 0 && item.PlaybackPositionTicks > 0 {
			ls.gridItems[i].Progress = float64(item.PlaybackPositionTicks) / float64(item.RuntimeTicks)
		}
		ls.gridItems[i].Watched = item.Played
		ls.gridItems[i].Rating = float64(item.CommunityRating)

		url := ls.client.GetPosterURL(item.ID)
		if img := ls.imgCache.Get(url); img != nil {
			ls.gridItems[i].Image = img
		} else {
			itemID := item.ID
			ls.imgCache.LoadAsync(url, func(img *ebiten.Image) {
				ls.mu.Lock()
				defer ls.mu.Unlock()
				for j := range ls.gridItems {
					if ls.gridItems[j].ID == itemID {
						ls.gridItems[j].Image = img
						break
					}
				}
			})
		}
	}

	ls.loaded = true
	ls.loading = false
	ls.loadingMore = false
	ls.loadError = ""
	ls.mu.Unlock()
}

func (ls *LibraryScreen) loadMore() {
	ls.loadData(len(ls.items))
}

func (ls *LibraryScreen) Update() (*ScreenTransition, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	dir, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Mouse wheel scroll
	_, wy := MouseWheelDelta()
	if wy != 0 {
		ls.targetScrollY -= wy * 60
		if ls.targetScrollY < 0 {
			ls.targetScrollY = 0
		}
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && ls.errDisplay.HandleClick(mx, my, ls.loadError) {
		return nil, nil
	}
	if clicked && ls.loaded {
		for i := range ls.gridItems {
			col := i % ls.grid.Cols
			row := i / ls.grid.Cols
			baseY := float64(NavBarHeight+10) - ls.scrollY
			x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
			y := baseY + float64(row)*(PosterHeight+PosterGap+FontSizeCaption+8)
			if PointInRect(mx, my, x, y, PosterWidth, PosterHeight) {
				ls.grid.Focused = i
				if i < len(ls.items) && ls.OnItemSelected != nil {
					ls.OnItemSelected(ls.items[i])
				}
				return nil, nil
			}
		}
	}

	// Right-click: toggle watched state
	rmx, rmy, rclicked := MouseJustRightClicked()
	if rclicked && ls.loaded {
		for i := range ls.gridItems {
			col := i % ls.grid.Cols
			row := i / ls.grid.Cols
			baseY := float64(NavBarHeight+10) - ls.scrollY
			x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
			y := baseY + float64(row)*(PosterHeight+PosterGap+FontSizeCaption+8)
			if PointInRect(rmx, rmy, x, y, PosterWidth, PosterHeight) {
				if i < len(ls.items) {
					if ls.items[i].Played {
						go ls.client.MarkUnplayed(ls.items[i].ID)
					} else {
						go ls.client.MarkPlayed(ls.items[i].ID)
					}
					ls.items[i].Played = !ls.items[i].Played
					ls.gridItems[i].Watched = ls.items[i].Played
				}
				return nil, nil
			}
		}
	}

	if !ls.loaded {
		return nil, nil
	}

	if dir != DirNone {
		ls.grid.Update(dir)
		ls.ensureVisible()
	}

	// Infinite scroll: check if user is within 2 rows of the end
	if ls.loaded && !ls.loadingMore && len(ls.items) < ls.total {
		totalRows := (len(ls.items) + ls.grid.Cols - 1) / ls.grid.Cols
		focusedRow := ls.grid.FocusedRow()
		if totalRows-focusedRow <= 2 {
			ls.loadingMore = true
			go ls.loadMore()
		}
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

	if ls.loadError != "" && !ls.loaded {
		errX := float64(ScreenWidth)/2 - 300
		errY := float64(ScreenHeight)/2 - 20
		ls.errDisplay.Draw(dst, ls.loadError, errX, errY, FontSizeBody)
		DrawTextCentered(dst, "Press Esc to go back", float64(ScreenWidth)/2, float64(ScreenHeight)/2+20,
			FontSizeSmall, ColorTextMuted)
		return
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

	// Loading more indicator at bottom
	if ls.loadingMore {
		totalRows := (len(ls.items) + ls.grid.Cols - 1) / ls.grid.Cols
		bottomY := baseY + float64(totalRows)*(PosterHeight+PosterGap+FontSizeCaption+8) + 20
		DrawTextCentered(dst, "Loading more...", float64(ScreenWidth)/2, bottomY,
			FontSizeBody, ColorTextSecondary)
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
		DrawImageCover(dst, item.Image, x, y, PosterWidth, PosterHeight)
	} else {
		DrawFilledRoundRect(dst, float32(x), float32(y),
			float32(PosterWidth), float32(PosterHeight), 4, ColorSurface)
		DrawTextCentered(dst, item.Title, x+PosterWidth/2, y+PosterHeight/2,
			FontSizeSmall, ColorTextMuted)
	}

	// Progress bar at bottom of poster
	if item.Progress > 0 && item.Progress < 1.0 {
		barH := float32(4)
		barY := float32(y + PosterHeight - float64(barH))
		DrawFilledRoundRect(dst, float32(x), barY, float32(PosterWidth), barH, 0,
			ColorSurfaceHover)
		DrawFilledRoundRect(dst, float32(x), barY,
			float32(float64(PosterWidth)*item.Progress), barH, 0,
			ColorPrimary)
	}

	// Watched badge
	if item.Watched {
		DrawTextCentered(dst, "✓", x+PosterWidth-12, y+12, FontSizeSmall, ColorSuccess)
	}

	// Rating badge (top-left corner)
	if item.Rating > 0 {
		drawRatingBadge(dst, item.Rating, x, y)
	}

	titleColor := ColorTextSecondary
	if focused {
		titleColor = ColorText
	}
	title := truncateText(item.Title, PosterWidth, FontSizeCaption)
	DrawText(dst, title, x, y+PosterHeight+4, FontSizeCaption, titleColor)
}
