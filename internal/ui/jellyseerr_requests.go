package ui

import (
	"fmt"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyseerr"
)

var requestFilters = []string{"all", "pending", "approved", "declined"}
var requestFilterLabels = []string{"All", "Pending", "Approved", "Declined"}

// JellyseerrRequestsScreen shows a list of all media requests.
type JellyseerrRequestsScreen struct {
	client   *jellyseerr.Client
	imgCache *cache.ImageCache

	requests  []jellyseerr.MediaRequest
	gridItems []GridItem
	grid      *FocusGrid
	total     int

	filterIndex int
	focusMode   int // 0=filter tabs, 1=request list
	loading     bool
	loadError   string

	scrollY       float64
	targetScrollY float64

	filterRects []ButtonRect

	OnItemSelected func(result jellyseerr.SearchResult)
	OnSearch       func()

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

// Search button layout
const (
	reqsSearchBtnW = 100.0
	reqsSearchBtnH = 38.0
	reqsSearchBtnY = NavBarHeight + 12.0
)

func NewJellyseerrRequestsScreen(client *jellyseerr.Client, imgCache *cache.ImageCache) *JellyseerrRequestsScreen {
	cols := (ScreenWidth - SectionPadding*2) / (PosterWidth + PosterGap)
	return &JellyseerrRequestsScreen{
		client:   client,
		imgCache: imgCache,
		grid:     NewFocusGrid(cols, 0),
	}
}

func (jr *JellyseerrRequestsScreen) Name() string { return "My Requests" }

func (jr *JellyseerrRequestsScreen) OnEnter() {
	go jr.loadRequests()
}

func (jr *JellyseerrRequestsScreen) OnExit() {}

func (jr *JellyseerrRequestsScreen) loadRequests() {
	jr.mu.Lock()
	jr.loading = true
	jr.loadError = ""
	filter := requestFilters[jr.filterIndex]
	jr.mu.Unlock()

	requests, total, err := jr.client.GetRequests(filter, 40, 0)

	jr.mu.Lock()
	defer jr.mu.Unlock()
	jr.loading = false

	if err != nil {
		jr.loadError = "Failed to load requests: " + err.Error()
		return
	}

	jr.requests = requests
	jr.total = total
	jr.grid.SetTotal(len(requests))
	jr.grid.Focused = 0
	jr.scrollY = 0
	jr.targetScrollY = 0

	jr.gridItems = make([]GridItem, len(requests))
	for i, req := range requests {
		title := fmt.Sprintf("Request #%d", req.ID)
		var subtitle string
		if req.Type == "movie" {
			subtitle = "Movie"
		} else {
			subtitle = "TV"
		}
		subtitle += " \u2022 " + jellyseerr.RequestStatusLabel(req.Status)

		jr.gridItems[i] = GridItem{
			ID:            fmt.Sprintf("%d", req.ID),
			Title:         title,
			Subtitle:      subtitle,
			RequestStatus: jr.requestToMediaStatus(req.Status),
		}

		// Fetch poster and title from TMDB detail lookup
		if req.Media.TmdbID != 0 {
			idx := i
			mediaType := req.Type
			tmdbID := req.Media.TmdbID
			go jr.fetchDetailForRequest(idx, mediaType, tmdbID)
		}
	}
}

// fetchDetailForRequest fetches movie/TV detail by TMDB ID to get poster and title.
func (jr *JellyseerrRequestsScreen) fetchDetailForRequest(idx int, mediaType string, tmdbID int) {
	var posterPath, title string

	if mediaType == "movie" {
		detail, err := jr.client.GetMovie(tmdbID)
		if err != nil {
			return
		}
		posterPath = detail.PosterPath
		title = detail.Title
	} else {
		detail, err := jr.client.GetTV(tmdbID)
		if err != nil {
			return
		}
		posterPath = detail.PosterPath
		title = detail.Name
	}

	jr.mu.Lock()
	defer jr.mu.Unlock()

	if idx >= len(jr.gridItems) {
		return
	}

	if title != "" {
		jr.gridItems[idx].Title = title
	}

	if posterPath != "" {
		posterURL := "https://image.tmdb.org/t/p/w300" + posterPath
		if img := jr.imgCache.Get(posterURL); img != nil {
			jr.gridItems[idx].Image = img
		} else {
			jr.imgCache.LoadAsync(posterURL, func(img *ebiten.Image) {
				jr.mu.Lock()
				defer jr.mu.Unlock()
				if idx < len(jr.gridItems) {
					jr.gridItems[idx].Image = img
				}
			})
		}
	}
}

// Also update selectRequest to include poster path from detail lookup
func (jr *JellyseerrRequestsScreen) fetchPosterForSelect(req jellyseerr.MediaRequest) jellyseerr.SearchResult {
	result := jellyseerr.SearchResult{
		ID:        req.Media.TmdbID,
		MediaType: req.Type,
		MediaInfo: &jellyseerr.MediaInfo{
			Status: req.Media.Status,
		},
		PosterPath: req.Media.PosterPath,
	}

	// Try to get the title from the grid item
	for _, item := range jr.gridItems {
		if item.ID == fmt.Sprintf("%d", req.ID) && item.Title != fmt.Sprintf("Request #%d", req.ID) {
			if req.Type == "movie" {
				result.Title = item.Title
			} else {
				result.Name = item.Title
			}
			break
		}
	}

	return result
}

// requestToMediaStatus maps a request status to a media status for badge display.
func (jr *JellyseerrRequestsScreen) requestToMediaStatus(reqStatus int) int {
	switch reqStatus {
	case jellyseerr.RequestPending:
		return jellyseerr.StatusPending
	case jellyseerr.RequestApproved:
		return jellyseerr.StatusProcessing
	case jellyseerr.RequestDeclined:
		return 0 // no badge
	default:
		return 0
	}
}

func (jr *JellyseerrRequestsScreen) searchBtnX() float64 {
	return float64(ScreenWidth) - SectionPadding - reqsSearchBtnW
}

func (jr *JellyseerrRequestsScreen) Update() (*ScreenTransition, error) {
	jr.mu.Lock()
	defer jr.mu.Unlock()

	_, enter, back := InputState()

	if back {
		if jr.focusMode == 1 {
			jr.focusMode = 0
			return nil, nil
		}
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Mouse wheel scroll
	_, wy := MouseWheelDelta()
	if wy != 0 {
		jr.targetScrollY -= wy * 60
		if jr.targetScrollY < 0 {
			jr.targetScrollY = 0
		}
	}

	// Mouse click
	mx, my, clicked := MouseJustClicked()
	if clicked && jr.errDisplay.HandleClick(mx, my, jr.loadError) {
		return nil, nil
	}
	if clicked {
		// Check filter tabs
		for i, rect := range jr.filterRects {
			if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
				if i != jr.filterIndex {
					jr.filterIndex = i
					go jr.loadRequests()
				}
				jr.focusMode = 0
				return nil, nil
			}
		}
		// Check search button
		searchX := jr.searchBtnX()
		if PointInRect(mx, my, searchX, reqsSearchBtnY, reqsSearchBtnW, reqsSearchBtnH) {
			if jr.OnSearch != nil {
				jr.OnSearch()
			}
			return nil, nil
		}
		// Check grid items
		if len(jr.gridItems) > 0 {
			baseY := float64(NavBarHeight) + 110.0 - jr.scrollY
			for i := range jr.gridItems {
				col := i % jr.grid.Cols
				row := i / jr.grid.Cols
				x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
				iy := baseY + float64(row)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16)
				if PointInRect(mx, my, x, iy, PosterWidth, PosterHeight) {
					jr.focusMode = 1
					jr.grid.Focused = i
					jr.selectRequest(i)
					return nil, nil
				}
			}
		}
	}

	// Keyboard shortcut: / for search
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) && jr.focusMode != 1 {
		if jr.OnSearch != nil {
			jr.OnSearch()
		}
		return nil, nil
	}

	switch jr.focusMode {
	case 0: // filter tabs
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			return &ScreenTransition{Type: TransitionFocusNavBar}, nil
		}
		dir, _, _ := InputState()
		switch dir {
		case DirLeft:
			if jr.filterIndex > 0 {
				jr.filterIndex--
				go jr.loadRequests()
			}
		case DirRight:
			if jr.filterIndex < len(requestFilters)-1 {
				jr.filterIndex++
				go jr.loadRequests()
			}
		case DirDown:
			if len(jr.gridItems) > 0 {
				jr.focusMode = 1
			}
		}

	case 1: // request list
		dir, _, _ := InputState()
		if dir != DirNone {
			if dir == DirUp && jr.grid.FocusedRow() == 0 {
				jr.focusMode = 0
			} else {
				jr.grid.Update(dir)
			}
		}
		if enter {
			jr.selectRequest(jr.grid.Focused)
		}
	}

	return nil, nil
}

func (jr *JellyseerrRequestsScreen) selectRequest(idx int) {
	if idx >= len(jr.requests) || jr.OnItemSelected == nil {
		return
	}
	req := jr.requests[idx]
	result := jr.fetchPosterForSelect(req)
	jr.OnItemSelected(result)
}

func (jr *JellyseerrRequestsScreen) Draw(dst *ebiten.Image) {
	jr.mu.Lock()
	defer jr.mu.Unlock()

	jr.scrollY = Lerp(jr.scrollY, jr.targetScrollY, ScrollAnimSpeed)

	// Header (below navbar)
	DrawText(dst, "My Requests", SectionPadding, NavBarHeight+16, FontSizeTitle, ColorPrimary)

	// Search button (top right)
	searchX := float32(jr.searchBtnX())
	drawNavButton(dst, "Search", searchX, reqsSearchBtnY, reqsSearchBtnW, reqsSearchBtnH,
		false, // not focusable via keyboard on this screen (use / shortcut)
		func(d *ebiten.Image, cx, cy, r float32, c color.Color) { drawSearchIcon(d, cx, cy, r, c) },
		ColorPrimary)

	// Filter tabs
	tabY := float64(NavBarHeight) + 55.0
	jr.filterRects = make([]ButtonRect, len(requestFilterLabels))
	tabX := float64(SectionPadding)
	for i, label := range requestFilterLabels {
		w, _ := MeasureText(label, FontSizeBody)
		tabW := w + 16
		tabH := FontSizeBody + 12.0

		jr.filterRects[i] = ButtonRect{X: tabX, Y: tabY, W: tabW, H: tabH}

		if i == jr.filterIndex {
			if jr.focusMode == 0 {
				vector.DrawFilledRect(dst, float32(tabX-4), float32(tabY-4),
					float32(tabW+8), float32(tabH+8), ColorSurfaceHover, false)
			}
			DrawText(dst, label, tabX+8, tabY+4, FontSizeBody, ColorPrimary)
			vector.DrawFilledRect(dst, float32(tabX), float32(tabY+tabH),
				float32(tabW), 2, ColorPrimary, false)
		} else {
			DrawText(dst, label, tabX+8, tabY+4, FontSizeBody, ColorTextMuted)
		}
		tabX += tabW + 16
	}

	baseY := float64(NavBarHeight) + 110.0

	if jr.loading {
		DrawTextCentered(dst, "Loading requests...", float64(ScreenWidth)/2, baseY+100,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	if jr.loadError != "" {
		jr.errDisplay.Draw(dst, jr.loadError, SectionPadding, baseY+100, FontSizeBody)
		return
	}

	if len(jr.gridItems) == 0 {
		DrawTextCentered(dst, "No requests found", float64(ScreenWidth)/2, baseY+100,
			FontSizeHeading, ColorTextSecondary)
		return
	}

	// Total count
	DrawText(dst, fmt.Sprintf("%d requests", jr.total), SectionPadding, baseY-20, FontSizeSmall, ColorTextMuted)

	for i, item := range jr.gridItems {
		col := i % jr.grid.Cols
		row := i / jr.grid.Cols
		x := SectionPadding + float64(col)*(PosterWidth+PosterGap)
		iy := baseY + float64(row)*(PosterHeight+PosterGap+FontSizeSmall+FontSizeCaption+16) - jr.scrollY

		if iy+PosterHeight < 0 || iy > float64(ScreenHeight) {
			continue
		}

		isFocused := jr.focusMode == 1 && i == jr.grid.Focused
		drawPosterItem(dst, item, x, iy, isFocused)
	}

}
