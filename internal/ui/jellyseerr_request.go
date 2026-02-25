package ui

import (
	"fmt"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/jellyseerr"
)

// JellyseerrRequestScreen shows details of a Jellyseerr search result and allows requesting it.
type JellyseerrRequestScreen struct {
	client   *jellyseerr.Client
	imgCache *cache.ImageCache
	result   jellyseerr.SearchResult

	poster *ebiten.Image

	// For TV shows — season selection
	tvDetail        *jellyseerr.TVDetail
	selectedSeasons []bool
	seasonFocused   int

	// Focus mode: 0=buttons, 1=season selection
	focusMode   int
	buttonIndex int
	buttons     []string
	buttonRects []ButtonRect

	status     int // media status
	requesting bool
	reqError   string
	reqSuccess string

	wantBack   bool
	trailerURL string

	// Callbacks
	OnPlayTrailer func(url string)

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

func NewJellyseerrRequestScreen(client *jellyseerr.Client, imgCache *cache.ImageCache, result jellyseerr.SearchResult) *JellyseerrRequestScreen {
	jr := &JellyseerrRequestScreen{
		client: client,
		imgCache: imgCache,
		result: result,
	}

	if result.MediaInfo != nil {
		jr.status = result.MediaInfo.Status
	}

	jr.updateButtons()
	return jr
}

func (jr *JellyseerrRequestScreen) Name() string {
	return "Request: " + jr.result.DisplayTitle()
}

func (jr *JellyseerrRequestScreen) OnEnter() {
	// Load poster
	posterURL := jr.result.PosterURL()
	if posterURL != "" {
		if img := jr.imgCache.Get(posterURL); img != nil {
			jr.poster = img
		} else {
			jr.imgCache.LoadAsync(posterURL, func(img *ebiten.Image) {
				jr.mu.Lock()
				jr.poster = img
				jr.mu.Unlock()
			})
		}
	}

	// Load detail for trailer info + season selection
	if jr.result.MediaType == "tv" {
		go jr.loadTVDetail()
	} else if jr.result.MediaType == "movie" {
		go jr.loadMovieDetail()
	}
}

func (jr *JellyseerrRequestScreen) OnExit() {}

func (jr *JellyseerrRequestScreen) loadTVDetail() {
	detail, err := jr.client.GetTV(jr.result.ID)
	if err != nil {
		log.Printf("Failed to load TV detail: %v", err)
		return
	}
	jr.mu.Lock()
	jr.tvDetail = detail
	if detail.MediaInfo != nil && detail.MediaInfo.Status > jr.status {
		jr.status = detail.MediaInfo.Status
	}
	jr.trailerURL = detail.TrailerURL()
	// Initialize season selection (all selected by default, skip specials)
	jr.selectedSeasons = make([]bool, len(detail.Seasons))
	for i, s := range detail.Seasons {
		jr.selectedSeasons[i] = s.SeasonNumber > 0
	}
	jr.updateButtons()
	jr.mu.Unlock()
}

func (jr *JellyseerrRequestScreen) loadMovieDetail() {
	detail, err := jr.client.GetMovie(jr.result.ID)
	if err != nil {
		log.Printf("Failed to load movie detail: %v", err)
		return
	}
	jr.mu.Lock()
	if detail.MediaInfo != nil && detail.MediaInfo.Status > jr.status {
		jr.status = detail.MediaInfo.Status
	}
	jr.trailerURL = detail.TrailerURL()
	jr.updateButtons()
	jr.mu.Unlock()
}

func (jr *JellyseerrRequestScreen) updateButtons() {
	jr.buttons = nil
	if jr.status < jellyseerr.StatusPending {
		jr.buttons = append(jr.buttons, "Request")
	}
	if jr.trailerURL != "" {
		jr.buttons = append(jr.buttons, "Trailer")
	}
	jr.buttons = append(jr.buttons, "Back")
	if jr.buttonIndex >= len(jr.buttons) {
		jr.buttonIndex = 0
	}
}

func (jr *JellyseerrRequestScreen) Update() (*ScreenTransition, error) {
	jr.mu.Lock()
	defer jr.mu.Unlock()

	dir, enter, back := InputState()

	if back || jr.wantBack {
		jr.wantBack = false
		return &ScreenTransition{Type: TransitionPop}, nil
	}

	// Mouse click handling
	mx, my, clicked := MouseJustClicked()
	if clicked && jr.errDisplay.HandleClick(mx, my, jr.reqError) {
		return nil, nil
	}
	if clicked {
		for i, rect := range jr.buttonRects {
			if PointInRect(mx, my, rect.X, rect.Y, rect.W, rect.H) {
				jr.buttonIndex = i
				jr.handleButton()
				return nil, nil
			}
		}
	}

	switch jr.focusMode {
	case 0: // buttons
		switch dir {
		case DirLeft:
			if jr.buttonIndex > 0 {
				jr.buttonIndex--
			}
		case DirRight:
			if jr.buttonIndex < len(jr.buttons)-1 {
				jr.buttonIndex++
			}
		case DirDown:
			if jr.tvDetail != nil && len(jr.tvDetail.Seasons) > 0 && jr.status < jellyseerr.StatusPending {
				jr.focusMode = 1
			}
		}
		if enter {
			jr.handleButton()
		}

	case 1: // season selection
		switch dir {
		case DirUp:
			if jr.seasonFocused > 0 {
				jr.seasonFocused--
			} else {
				jr.focusMode = 0
			}
		case DirDown:
			if jr.seasonFocused < len(jr.tvDetail.Seasons)-1 {
				jr.seasonFocused++
			}
		}
		if enter {
			jr.selectedSeasons[jr.seasonFocused] = !jr.selectedSeasons[jr.seasonFocused]
		}
	}

	return nil, nil
}

func (jr *JellyseerrRequestScreen) handleButton() {
	btn := jr.buttons[jr.buttonIndex]
	switch btn {
	case "Request":
		if jr.requesting {
			return
		}
		go jr.doRequest()
	case "Trailer":
		if jr.OnPlayTrailer != nil && jr.trailerURL != "" {
			jr.OnPlayTrailer(jr.trailerURL)
		}
	case "Back":
		jr.wantBack = true
	}
}

func (jr *JellyseerrRequestScreen) doRequest() {
	jr.mu.Lock()
	jr.requesting = true
	jr.reqError = ""
	jr.reqSuccess = ""

	mediaType := jr.result.MediaType
	var seasons []int
	if mediaType == "tv" && jr.tvDetail != nil {
		for i, sel := range jr.selectedSeasons {
			if sel {
				seasons = append(seasons, jr.tvDetail.Seasons[i].SeasonNumber)
			}
		}
		if len(seasons) == 0 {
			jr.reqError = "Select at least one season"
			jr.requesting = false
			jr.mu.Unlock()
			return
		}
	}
	jr.mu.Unlock()

	_, err := jr.client.CreateRequest(jr.result.ID, mediaType, seasons)

	jr.mu.Lock()
	defer jr.mu.Unlock()
	jr.requesting = false

	if err != nil {
		jr.reqError = "Request failed: " + err.Error()
		return
	}

	jr.reqSuccess = "Request submitted!"
	jr.status = jellyseerr.StatusPending
	jr.updateButtons()
}

func (jr *JellyseerrRequestScreen) Draw(dst *ebiten.Image) {
	jr.mu.Lock()
	defer jr.mu.Unlock()

	// Background
	vector.DrawFilledRect(dst, 0, 0, float32(ScreenWidth), float32(ScreenHeight), ColorBackground, false)

	x := float64(SectionPadding)
	y := 30.0

	// Poster on the left
	posterX := x
	posterY := y
	posterW := 200.0
	posterH := 300.0
	if jr.poster != nil {
		DrawImageCover(dst, jr.poster, posterX, posterY, posterW, posterH)
	} else {
		vector.DrawFilledRect(dst, float32(posterX), float32(posterY),
			float32(posterW), float32(posterH), ColorSurface, false)
		DrawTextCentered(dst, jr.result.DisplayTitle(), posterX+posterW/2, posterY+posterH/2,
			FontSizeSmall, ColorTextMuted)
	}

	// Info on the right of poster
	infoX := posterX + posterW + 30
	infoY := posterY

	DrawText(dst, jr.result.DisplayTitle(), infoX, infoY, FontSizeTitle, ColorText)
	infoY += FontSizeTitle + 8

	// Year + type
	meta := ""
	if yr := jr.result.Year(); yr != "" {
		meta = yr
	}
	if jr.result.MediaType != "" {
		if meta != "" {
			meta += "  \u2022  "
		}
		if jr.result.MediaType == "tv" {
			meta += "TV Series"
		} else {
			meta += "Movie"
		}
	}
	if meta != "" {
		DrawText(dst, meta, infoX, infoY, FontSizeBody, ColorTextSecondary)
		infoY += FontSizeBody + 12
	}

	// Status badge
	statusLabel := jellyseerr.MediaStatusLabel(jr.status)
	if statusLabel != "" {
		badgeColor := statusBadgeColor(jr.status)
		tw, _ := MeasureText(statusLabel, FontSizeSmall)
		vector.DrawFilledRect(dst, float32(infoX), float32(infoY),
			float32(tw+16), float32(FontSizeSmall+8), badgeColor, false)
		DrawText(dst, statusLabel, infoX+8, infoY+4, FontSizeSmall, ColorText)
		infoY += FontSizeSmall + 20
	} else {
		infoY += 8
	}

	// Overview
	if jr.result.Overview != "" {
		maxW := float64(ScreenWidth) - infoX - SectionPadding
		h := DrawTextWrapped(dst, jr.result.Overview, infoX, infoY, maxW, FontSizeBody, ColorTextSecondary)
		infoY += h + 16
	}

	// Error / success messages
	if jr.reqError != "" {
		infoY += jr.errDisplay.Draw(dst, jr.reqError, infoX, infoY, FontSizeSmall)
	}
	if jr.reqSuccess != "" {
		DrawText(dst, jr.reqSuccess, infoX, infoY, FontSizeSmall, ColorSuccess)
		infoY += FontSizeSmall + 8
	}

	// Buttons
	jr.buttonRects = make([]ButtonRect, len(jr.buttons))
	btnX := infoX
	btnY := infoY + 8
	for i, label := range jr.buttons {
		tw, _ := MeasureText(label, FontSizeBody)
		w := tw + 40
		h := 36.0

		jr.buttonRects[i] = ButtonRect{X: btnX, Y: btnY, W: w, H: h}

		if jr.focusMode == 0 && i == jr.buttonIndex {
			vector.DrawFilledRect(dst, float32(btnX), float32(btnY), float32(w), float32(h), ColorPrimary, false)
			DrawTextCentered(dst, label, btnX+w/2, btnY+h/2, FontSizeBody, ColorText)
		} else {
			vector.DrawFilledRect(dst, float32(btnX), float32(btnY), float32(w), float32(h), ColorSurface, false)
			DrawTextCentered(dst, label, btnX+w/2, btnY+h/2, FontSizeBody, ColorTextSecondary)
		}
		btnX += w + 12
	}

	if jr.requesting {
		DrawText(dst, "Requesting...", btnX+20, btnY+8, FontSizeSmall, ColorTextSecondary)
	}

	// Season selection for TV shows
	if jr.tvDetail != nil && len(jr.tvDetail.Seasons) > 0 && jr.status < jellyseerr.StatusPending {
		seasonY := posterY + posterH + 30
		DrawText(dst, "Select Seasons:", x, seasonY, FontSizeHeading, ColorText)
		seasonY += FontSizeHeading + 8

		for i, season := range jr.tvDetail.Seasons {
			isFocused := jr.focusMode == 1 && i == jr.seasonFocused
			rowH := 32.0

			if isFocused {
				vector.DrawFilledRect(dst, float32(x-8), float32(seasonY-4),
					float32(500), float32(rowH+4), ColorSurfaceHover, false)
			}

			check := "[ ]"
			if jr.selectedSeasons[i] {
				check = "[x]"
			}

			label := fmt.Sprintf("%s  %s", check, season.Name)
			if season.Name == "" {
				label = fmt.Sprintf("%s  Season %d", check, season.SeasonNumber)
			}
			if season.EpisodeCount > 0 {
				label += fmt.Sprintf(" (%d episodes)", season.EpisodeCount)
			}

			clr := ColorTextSecondary
			if isFocused {
				clr = ColorText
			}
			DrawText(dst, label, x, seasonY, FontSizeBody, clr)
			seasonY += rowH
		}
	}

	// Hint
	hint := "Enter to select  \u00b7  Esc to go back"
	DrawText(dst, hint, SectionPadding, float64(ScreenHeight)-40, FontSizeSmall, ColorTextMuted)
}
