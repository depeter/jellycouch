package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyfin"
)

const (
	focusGrid      = 0
	focusFilterBar = 1
)

// Sort option mappings
var sortOptions = []struct {
	Label     string
	SortBy    string
	SortOrder string
}{
	{"Name A-Z", "SortName", "Ascending"},
	{"Name Z-A", "SortName", "Descending"},
	{"Date Added (New)", "DateCreated", "Descending"},
	{"Date Added (Old)", "DateCreated", "Ascending"},
	{"Release Date (New)", "PremiereDate", "Descending"},
	{"Release Date (Old)", "PremiereDate", "Ascending"},
	{"Rating (High)", "CommunityRating", "Descending"},
	{"Rating (Low)", "CommunityRating", "Ascending"},
	{"Random", "Random", "Ascending"},
}

// Status option mappings
var statusOptions = []struct {
	Label  string
	Filter string
}{
	{"All", ""},
	{"Unplayed", "IsUnplayed"},
	{"Played", "IsPlayed"},
	{"Favorites", "IsFavorite"},
	{"Resumable", "IsResumable"},
}

// Letter filter options (A-Z + # for non-alpha)
var letterOptions = []string{
	"All", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M",
	"N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "#",
}

// Year/decade filter options
var yearOptions = []struct {
	Label string
	Years []int32
}{
	{"All", nil},
	{"2020s", yearsRange(2020, 2029)},
	{"2010s", yearsRange(2010, 2019)},
	{"2000s", yearsRange(2000, 2009)},
	{"1990s", yearsRange(1990, 1999)},
	{"1980s", yearsRange(1980, 1989)},
	{"1970s", yearsRange(1970, 1979)},
	{"Older", yearsRange(1900, 1969)},
}

func yearsRange(from, to int) []int32 {
	years := make([]int32, 0, to-from+1)
	for y := from; y <= to; y++ {
		years = append(years, int32(y))
	}
	return years
}

// LibraryScreen shows a grid of all items in a library with filtering and sorting.
type LibraryScreen struct {
	client   *jellyfin.Client
	imgCache *cache.ImageCache

	parentID  string
	title     string
	itemTypes []string

	items     []jellyfin.MediaItem
	grid      *FocusGrid
	gridItems []GridItem
	total     int

	filterBar  *FilterBar
	filter     jellyfin.LibraryFilter
	genres     []string
	genresLoaded bool
	focusMode  int // focusGrid or focusFilterBar

	loaded      bool
	loading     bool
	loadingMore bool
	loadError   string
	scrollY     float64
	targetScrollY float64

	// debounce: track last applied search to detect changes
	appliedSearch string

	OnItemSelected func(item jellyfin.MediaItem)

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

func NewLibraryScreen(client *jellyfin.Client, imgCache *cache.ImageCache, parentID, title string, itemTypes []string) *LibraryScreen {
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)

	// Build sort option labels
	sortLabels := make([]string, len(sortOptions))
	for i, opt := range sortOptions {
		sortLabels[i] = opt.Label
	}

	// Build status option labels
	statusLabels := make([]string, len(statusOptions))
	for i, opt := range statusOptions {
		statusLabels[i] = opt.Label
	}

	// Build year option labels
	yearLabels := make([]string, len(yearOptions))
	for i, opt := range yearOptions {
		yearLabels[i] = opt.Label
	}

	filterBar := NewFilterBar([]FilterOption{
		{Label: "Sort", Options: sortLabels, Selected: 2},       // Date Added (New)
		{Label: "Genre", Options: []string{"All"}, Selected: 0},
		{Label: "Status", Options: statusLabels, Selected: 1}, // Unplayed
		{Label: "Letter", Options: letterOptions, Selected: 0},
		{Label: "Year", Options: yearLabels, Selected: 0},
	})

	ls := &LibraryScreen{
		client:    client,
		imgCache:  imgCache,
		parentID:  parentID,
		title:     title,
		itemTypes: itemTypes,
		grid:      NewFocusGrid(cols, 0),
		filterBar: filterBar,
		focusMode: focusGrid,
	}
	ls.filter = ls.buildFilter()
	return ls
}

func (ls *LibraryScreen) Name() string { return "Library: " + ls.title }

func (ls *LibraryScreen) OnEnter() {
	if !ls.loaded && !ls.loading {
		ls.loading = true
		go ls.loadData(0)
		go ls.loadGenres()
	}
}

func (ls *LibraryScreen) OnExit() {}

func (ls *LibraryScreen) loadGenres() {
	genres, err := ls.client.GetGenres(ls.parentID, ls.itemTypes)
	if err != nil {
		log.Printf("Failed to load genres: %v", err)
		return
	}

	ls.mu.Lock()
	ls.genres = genres
	ls.genresLoaded = true

	// Rebuild genre pill options: "All" + genre names
	genreOptions := make([]string, 0, len(genres)+1)
	genreOptions = append(genreOptions, "All")
	genreOptions = append(genreOptions, genres...)
	if len(ls.filterBar.Filters) > 1 {
		ls.filterBar.Filters[1].Options = genreOptions
		// Reset selection if it's out of range
		if ls.filterBar.Filters[1].Selected >= len(genreOptions) {
			ls.filterBar.Filters[1].Selected = 0
		}
	}
	ls.mu.Unlock()
}

func (ls *LibraryScreen) buildFilter() jellyfin.LibraryFilter {
	f := jellyfin.LibraryFilter{}

	// Sort
	sortIdx := ls.filterBar.Filters[0].Selected
	if sortIdx >= 0 && sortIdx < len(sortOptions) {
		f.SortBy = sortOptions[sortIdx].SortBy
		f.SortOrder = sortOptions[sortIdx].SortOrder
	}

	// Genre
	genreVal := ls.filterBar.Filters[1].Value()
	if genreVal != "" && genreVal != "All" {
		f.Genres = []string{genreVal}
	}

	// Status
	statusIdx := ls.filterBar.Filters[2].Selected
	if statusIdx >= 0 && statusIdx < len(statusOptions) {
		f.Status = statusOptions[statusIdx].Filter
	}

	// Letter
	letterVal := ls.filterBar.Filters[3].Value()
	if letterVal != "" && letterVal != "All" {
		f.Letter = letterVal
	}

	// Year
	yearIdx := ls.filterBar.Filters[4].Selected
	if yearIdx >= 0 && yearIdx < len(yearOptions) {
		f.Years = yearOptions[yearIdx].Years
	}

	// Search
	f.Search = ls.filterBar.SearchInput.Text

	return f
}

func (ls *LibraryScreen) applyFilters() {
	ls.filter = ls.buildFilter()
	ls.appliedSearch = ls.filterBar.SearchInput.Text
	ls.items = nil
	ls.gridItems = nil
	ls.total = 0
	ls.grid.Focused = 0
	ls.grid.SetTotal(0)
	ls.scrollY = 0
	ls.targetScrollY = 0
	ls.loaded = false
	ls.loading = true
	ls.loadError = ""
	go ls.loadData(0)
}

func (ls *LibraryScreen) loadData(start int) {
	ls.mu.Lock()
	filter := ls.filter
	ls.mu.Unlock()

	items, total, err := ls.client.GetFilteredItems(ls.parentID, start, 50, ls.itemTypes, filter)
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
		if item.Type == "Episode" && item.SeriesName != "" {
			ls.gridItems[i].Title = item.SeriesName
			ep := fmt.Sprintf("S%dE%d", item.ParentIndexNumber, item.IndexNumber)
			if item.Name != "" {
				ep += " · " + item.Name
			}
			ls.gridItems[i].Subtitle = ep
		} else if item.Year > 0 {
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

// gridBaseY returns the Y position where the poster grid starts.
func (ls *LibraryScreen) gridBaseY() float64 {
	return float64(NavBarHeight) + filterBarHeight + 20
}

func (ls *LibraryScreen) Update() (*ScreenTransition, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

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

	// Filter bar mouse click
	if clicked {
		if idx, ok := ls.filterBar.HandleClick(mx, my); ok {
			ls.focusMode = focusFilterBar
			ls.filterBar.Active = true
			if idx < len(ls.filterBar.Filters) {
				ls.filterBar.FocusedIndex = idx
				// Cycle pill value on click
				pill := &ls.filterBar.Filters[idx]
				pill.Selected = (pill.Selected + 1) % len(pill.Options)
				ls.applyFilters()
			} else {
				ls.filterBar.FocusedIndex = idx
			}
			return nil, nil
		}
	}

	// Grid mouse click
	if clicked && ls.loaded {
		gridBase := ls.gridBaseY() - ls.scrollY
		for i := range ls.gridItems {
			col := i % ls.grid.Cols
			row := i / ls.grid.Cols
			x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
			y := gridBase + float64(row)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16)
			if PointInRect(mx, my, x, y, PosterWidth, PosterHeight) {
				ls.focusMode = focusGrid
				ls.filterBar.Active = false
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
		gridBase := ls.gridBaseY() - ls.scrollY
		for i := range ls.gridItems {
			col := i % ls.grid.Cols
			row := i / ls.grid.Cols
			x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
			y := gridBase + float64(row)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16)
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

	// Focus mode dispatch
	if ls.focusMode == focusFilterBar {
		return ls.updateFilterBar()
	}
	return ls.updateGrid()
}

func (ls *LibraryScreen) updateFilterBar() (*ScreenTransition, error) {
	// Escape/Down from filter bar → return to grid
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ls.focusMode = focusGrid
		ls.filterBar.Active = false
		return nil, nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && !ls.filterBar.IsSearchFocused() {
		ls.focusMode = focusGrid
		ls.filterBar.Active = false
		return nil, nil
	}

	changed := ls.filterBar.Update()

	// Check if search text changed and apply debounced
	if ls.filterBar.SearchInput.Text != ls.appliedSearch {
		// Apply on Enter (handled inside filterBar.Update returning changed=true)
		if changed {
			ls.applyFilters()
			return nil, nil
		}
	} else if changed {
		ls.applyFilters()
	}

	return nil, nil
}

func (ls *LibraryScreen) updateGrid() (*ScreenTransition, error) {
	dir, enter, back := InputState()

	if back {
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Shortcut keys
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		ls.focusMode = focusFilterBar
		ls.filterBar.Active = true
		ls.filterBar.FocusedIndex = len(ls.filterBar.Filters) // search input
		return nil, nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ls.focusMode = focusFilterBar
		ls.filterBar.Active = true
		ls.filterBar.FocusedIndex = 0
		return nil, nil
	}

	if !ls.loaded {
		return nil, nil
	}

	if dir != DirNone {
		// Check if Up on first row → go to filter bar
		if dir == DirUp && ls.grid.FocusedRow() == 0 {
			ls.focusMode = focusFilterBar
			ls.filterBar.Active = true
			if ls.filterBar.FocusedIndex >= len(ls.filterBar.Filters) {
				ls.filterBar.FocusedIndex = 0
			}
			return nil, nil
		}
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

	// Filter bar
	ls.filterBar.Draw(dst, SectionPadding, float64(NavBarHeight))

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

	if len(ls.gridItems) == 0 {
		DrawTextCentered(dst, "No items found", float64(ScreenWidth)/2, float64(ScreenHeight)/2,
			FontSizeHeading, ColorTextSecondary)

		return
	}

	// Draw grid
	baseY := ls.gridBaseY() - ls.scrollY
	for i, item := range ls.gridItems {
		col := i % ls.grid.Cols
		row := i / ls.grid.Cols

		x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
		y := baseY + float64(row)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16)

		// Skip offscreen
		if y+PosterHeight < 0 || y > float64(ScreenHeight) {
			continue
		}

		isFocused := ls.focusMode == focusGrid && i == ls.grid.Focused
		drawPosterItem(dst, item, x, y, isFocused)
	}

	// Loading more indicator at bottom
	if ls.loadingMore {
		totalRows := (len(ls.items) + ls.grid.Cols - 1) / ls.grid.Cols
		bottomY := baseY + float64(totalRows)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16) + 20
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
		drawCheckmark(dst, float32(x+PosterWidth-12), float32(y+12), 5, ColorSuccess)
	}

	// Rating badge (top-left corner)
	if item.Rating > 0 {
		drawRatingBadge(dst, item.Rating, x, y)
	}

	titleColor := ColorTextSecondary
	if focused {
		titleColor = ColorText
	}
	title := truncateText(item.Title, PosterWidth, FontSizeSmall)
	DrawText(dst, title, x, y+PosterHeight+6, FontSizeSmall, titleColor)

	// Subtitle below title
	if item.Subtitle != "" {
		sub := truncateText(item.Subtitle, PosterWidth, FontSizeCaption)
		DrawText(dst, sub, x, y+PosterHeight+6+FontSizeSmall+4, FontSizeCaption, ColorTextMuted)
	}
}
