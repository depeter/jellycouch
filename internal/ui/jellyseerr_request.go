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

	// Request options (quality, root folder, language, 4K)
	radarrServers   []jellyseerr.RadarrSettings
	sonarrServers   []jellyseerr.SonarrSettings
	selectedServer  int
	selectedProfile int
	selectedFolder  int
	selectedLang    int
	is4K            bool
	servicesLoaded  bool
	optionIndex     int // focused option row

	// Focus mode: 0=buttons, 1=request options, 2=season selection
	focusMode   int
	buttonIndex int
	buttons     []string
	buttonRects []ButtonRect

	status     int // media status
	requesting bool
	reqError   string
	reqSuccess string

	wantBack    bool
	trailerURL  string
	voteAverage float64

	// Callbacks
	OnPlayTrailer func(url string)

	errDisplay ErrorDisplay
	mu         sync.Mutex
}

func NewJellyseerrRequestScreen(client *jellyseerr.Client, imgCache *cache.ImageCache, result jellyseerr.SearchResult) *JellyseerrRequestScreen {
	jr := &JellyseerrRequestScreen{
		client:   client,
		imgCache: imgCache,
		result:   result,
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

	// Load service settings for request options
	if jr.status < jellyseerr.StatusPending {
		go jr.loadServiceSettings()
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
	jr.voteAverage = detail.VoteAverage
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
	jr.voteAverage = detail.VoteAverage
	if detail.MediaInfo != nil && detail.MediaInfo.Status > jr.status {
		jr.status = detail.MediaInfo.Status
	}
	jr.trailerURL = detail.TrailerURL()
	jr.updateButtons()
	jr.mu.Unlock()
}

func (jr *JellyseerrRequestScreen) loadServiceSettings() {
	if jr.result.MediaType == "movie" {
		settings, err := jr.client.GetRadarrSettings()
		if err != nil {
			log.Printf("Failed to load Radarr settings: %v", err)
			return
		}
		jr.mu.Lock()
		jr.radarrServers = settings
		jr.preselectRadarrDefaults()
		jr.servicesLoaded = true
		jr.mu.Unlock()
	} else {
		settings, err := jr.client.GetSonarrSettings()
		if err != nil {
			log.Printf("Failed to load Sonarr settings: %v", err)
			return
		}
		jr.mu.Lock()
		jr.sonarrServers = settings
		jr.preselectSonarrDefaults()
		jr.servicesLoaded = true
		jr.mu.Unlock()
	}
}

func (jr *JellyseerrRequestScreen) preselectRadarrDefaults() {
	// Find default non-4K server
	for i, s := range jr.radarrServers {
		if s.IsDefault && !s.Is4K {
			jr.selectedServer = i
			break
		}
	}
	if jr.selectedServer >= len(jr.radarrServers) {
		return
	}
	srv := jr.radarrServers[jr.selectedServer]
	// Pre-select active profile
	for i, p := range srv.Profiles {
		if p.ID == srv.ActiveProfileID {
			jr.selectedProfile = i
			break
		}
	}
	// Pre-select active root folder
	for i, f := range srv.RootFolders {
		if f.Path == srv.ActiveDirectory {
			jr.selectedFolder = i
			break
		}
	}
}

func (jr *JellyseerrRequestScreen) preselectSonarrDefaults() {
	// Find default non-4K server
	for i, s := range jr.sonarrServers {
		if s.IsDefault && !s.Is4K {
			jr.selectedServer = i
			break
		}
	}
	if jr.selectedServer >= len(jr.sonarrServers) {
		return
	}
	srv := jr.sonarrServers[jr.selectedServer]
	for i, p := range srv.Profiles {
		if p.ID == srv.ActiveProfileID {
			jr.selectedProfile = i
			break
		}
	}
	for i, f := range srv.RootFolders {
		if f.Path == srv.ActiveDirectory {
			jr.selectedFolder = i
			break
		}
	}
	for i, l := range srv.LanguageProfiles {
		if l.ID == srv.ActiveLanguageProfileID {
			jr.selectedLang = i
			break
		}
	}
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

// optionRowCount returns the number of option rows visible.
func (jr *JellyseerrRequestScreen) optionRowCount() int {
	if !jr.servicesLoaded || jr.status >= jellyseerr.StatusPending {
		return 0
	}
	if jr.result.MediaType == "movie" {
		count := 0
		srv := jr.activeRadarr()
		if srv == nil {
			return 0
		}
		if jr.hasMultipleRadarrServers() {
			count++ // server
		}
		if len(srv.Profiles) > 0 {
			count++ // profile
		}
		if len(srv.RootFolders) > 0 {
			count++ // folder
		}
		count++ // 4K toggle
		return count
	}
	// TV
	count := 0
	srv := jr.activeSonarr()
	if srv == nil {
		return 0
	}
	if jr.hasMultipleSonarrServers() {
		count++
	}
	if len(srv.Profiles) > 0 {
		count++
	}
	if len(srv.RootFolders) > 0 {
		count++
	}
	if len(srv.LanguageProfiles) > 0 {
		count++
	}
	count++ // 4K toggle
	return count
}

func (jr *JellyseerrRequestScreen) hasMultipleRadarrServers() bool {
	n := 0
	for _, s := range jr.radarrServers {
		if !s.Is4K {
			n++
		}
	}
	return n > 1
}

func (jr *JellyseerrRequestScreen) hasMultipleSonarrServers() bool {
	n := 0
	for _, s := range jr.sonarrServers {
		if !s.Is4K {
			n++
		}
	}
	return n > 1
}

func (jr *JellyseerrRequestScreen) activeRadarr() *jellyseerr.RadarrSettings {
	if jr.selectedServer < len(jr.radarrServers) {
		return &jr.radarrServers[jr.selectedServer]
	}
	return nil
}

func (jr *JellyseerrRequestScreen) activeSonarr() *jellyseerr.SonarrSettings {
	if jr.selectedServer < len(jr.sonarrServers) {
		return &jr.sonarrServers[jr.selectedServer]
	}
	return nil
}

// hasSeasonSelection returns true if season selection should be shown.
func (jr *JellyseerrRequestScreen) hasSeasonSelection() bool {
	return jr.tvDetail != nil && len(jr.tvDetail.Seasons) > 0 && jr.status < jellyseerr.StatusPending
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
			if jr.optionRowCount() > 0 {
				jr.focusMode = 1
				jr.optionIndex = 0
			} else if jr.hasSeasonSelection() {
				jr.focusMode = 2
			}
		}
		if enter {
			jr.handleButton()
		}

	case 1: // request options
		switch dir {
		case DirUp:
			if jr.optionIndex > 0 {
				jr.optionIndex--
			} else {
				jr.focusMode = 0
			}
		case DirDown:
			if jr.optionIndex < jr.optionRowCount()-1 {
				jr.optionIndex++
			} else if jr.hasSeasonSelection() {
				jr.focusMode = 2
				jr.seasonFocused = 0
			}
		case DirLeft:
			jr.cycleOption(-1)
		case DirRight:
			jr.cycleOption(1)
		}

	case 2: // season selection
		switch dir {
		case DirUp:
			if jr.seasonFocused > 0 {
				jr.seasonFocused--
			} else if jr.optionRowCount() > 0 {
				jr.focusMode = 1
				jr.optionIndex = jr.optionRowCount() - 1
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

// optionRowType returns a label identifying what the given row index represents.
// The order must match the draw order.
func (jr *JellyseerrRequestScreen) optionRowType(row int) string {
	cur := 0
	if jr.result.MediaType == "movie" {
		srv := jr.activeRadarr()
		if srv == nil {
			return ""
		}
		if jr.hasMultipleRadarrServers() {
			if row == cur {
				return "server"
			}
			cur++
		}
		if len(srv.Profiles) > 0 {
			if row == cur {
				return "profile"
			}
			cur++
		}
		if len(srv.RootFolders) > 0 {
			if row == cur {
				return "folder"
			}
			cur++
		}
		if row == cur {
			return "4k"
		}
		return ""
	}
	// TV
	srv := jr.activeSonarr()
	if srv == nil {
		return ""
	}
	if jr.hasMultipleSonarrServers() {
		if row == cur {
			return "server"
		}
		cur++
	}
	if len(srv.Profiles) > 0 {
		if row == cur {
			return "profile"
		}
		cur++
	}
	if len(srv.RootFolders) > 0 {
		if row == cur {
			return "folder"
		}
		cur++
	}
	if len(srv.LanguageProfiles) > 0 {
		if row == cur {
			return "language"
		}
		cur++
	}
	if row == cur {
		return "4k"
	}
	return ""
}

func (jr *JellyseerrRequestScreen) cycleOption(delta int) {
	kind := jr.optionRowType(jr.optionIndex)
	switch kind {
	case "server":
		if jr.result.MediaType == "movie" {
			jr.selectedServer = wrapIndex(jr.selectedServer+delta, len(jr.radarrServers))
			jr.selectedProfile = 0
			jr.selectedFolder = 0
			jr.preselectRadarrDefaults()
		} else {
			jr.selectedServer = wrapIndex(jr.selectedServer+delta, len(jr.sonarrServers))
			jr.selectedProfile = 0
			jr.selectedFolder = 0
			jr.selectedLang = 0
			jr.preselectSonarrDefaults()
		}
	case "profile":
		if jr.result.MediaType == "movie" {
			if srv := jr.activeRadarr(); srv != nil {
				jr.selectedProfile = wrapIndex(jr.selectedProfile+delta, len(srv.Profiles))
			}
		} else {
			if srv := jr.activeSonarr(); srv != nil {
				jr.selectedProfile = wrapIndex(jr.selectedProfile+delta, len(srv.Profiles))
			}
		}
	case "folder":
		if jr.result.MediaType == "movie" {
			if srv := jr.activeRadarr(); srv != nil {
				jr.selectedFolder = wrapIndex(jr.selectedFolder+delta, len(srv.RootFolders))
			}
		} else {
			if srv := jr.activeSonarr(); srv != nil {
				jr.selectedFolder = wrapIndex(jr.selectedFolder+delta, len(srv.RootFolders))
			}
		}
	case "language":
		if srv := jr.activeSonarr(); srv != nil {
			jr.selectedLang = wrapIndex(jr.selectedLang+delta, len(srv.LanguageProfiles))
		}
	case "4k":
		jr.is4K = !jr.is4K
	}
}

func wrapIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	return ((i % n) + n) % n
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

func (jr *JellyseerrRequestScreen) buildRequestOptions() *jellyseerr.RequestOptions {
	if !jr.servicesLoaded {
		return nil
	}
	opts := &jellyseerr.RequestOptions{
		Is4K: jr.is4K,
	}
	if jr.result.MediaType == "movie" {
		srv := jr.activeRadarr()
		if srv == nil {
			return nil
		}
		opts.ServerID = srv.ID
		if jr.selectedProfile < len(srv.Profiles) {
			opts.ProfileID = srv.Profiles[jr.selectedProfile].ID
		}
		if jr.selectedFolder < len(srv.RootFolders) {
			opts.RootFolder = srv.RootFolders[jr.selectedFolder].Path
		}
	} else {
		srv := jr.activeSonarr()
		if srv == nil {
			return nil
		}
		opts.ServerID = srv.ID
		if jr.selectedProfile < len(srv.Profiles) {
			opts.ProfileID = srv.Profiles[jr.selectedProfile].ID
		}
		if jr.selectedFolder < len(srv.RootFolders) {
			opts.RootFolder = srv.RootFolders[jr.selectedFolder].Path
		}
		if jr.selectedLang < len(srv.LanguageProfiles) {
			opts.LanguageProfileID = srv.LanguageProfiles[jr.selectedLang].ID
		}
	}
	return opts
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
	opts := jr.buildRequestOptions()
	jr.mu.Unlock()

	_, err := jr.client.CreateRequest(jr.result.ID, mediaType, seasons, opts)

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

	// Year + type + rating
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
	if jr.voteAverage > 0 {
		if meta != "" {
			meta += "  \u2022  "
		}
		meta += fmt.Sprintf("★ %.1f", jr.voteAverage)
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

	// --- Request options section (below poster area) ---
	optY := posterY + posterH + 30

	if jr.optionRowCount() > 0 {
		DrawText(dst, "Request Options:", x, optY, FontSizeHeading, ColorText)
		optY += FontSizeHeading + 8
		optY = jr.drawOptions(dst, x, optY)
		optY += 16
	}

	// Season selection for TV shows
	if jr.hasSeasonSelection() {
		DrawText(dst, "Select Seasons:", x, optY, FontSizeHeading, ColorText)
		optY += FontSizeHeading + 8

		for i, season := range jr.tvDetail.Seasons {
			isFocused := jr.focusMode == 2 && i == jr.seasonFocused
			rowH := 32.0

			if isFocused {
				vector.DrawFilledRect(dst, float32(x-8), float32(optY-4),
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
			DrawText(dst, label, x, optY, FontSizeBody, clr)
			optY += rowH
		}
	}

}

// drawOptions draws the option rows and returns the Y position after the last row.
func (jr *JellyseerrRequestScreen) drawOptions(dst *ebiten.Image, x, y float64) float64 {
	rowH := 32.0
	optW := 500.0

	for row := 0; row < jr.optionRowCount(); row++ {
		isFocused := jr.focusMode == 1 && row == jr.optionIndex
		kind := jr.optionRowType(row)

		if isFocused {
			vector.DrawFilledRect(dst, float32(x-8), float32(y-4),
				float32(optW+16), float32(rowH+4), ColorSurfaceHover, false)
		}

		label, value := jr.optionLabelValue(kind)
		labelClr := ColorTextSecondary
		valueClr := ColorTextSecondary
		if isFocused {
			labelClr = ColorText
			valueClr = ColorText
		}

		DrawText(dst, label, x, y+4, FontSizeBody, labelClr)

		// Value with arrows
		valueX := x + 200
		if isFocused {
			DrawText(dst, "\u25C0", valueX-20, y+4, FontSizeBody, ColorPrimary)
		}
		DrawText(dst, value, valueX, y+4, FontSizeBody, valueClr)
		if isFocused {
			tw, _ := MeasureText(value, FontSizeBody)
			DrawText(dst, "\u25B6", valueX+tw+8, y+4, FontSizeBody, ColorPrimary)
		}

		y += rowH
	}

	return y
}

func (jr *JellyseerrRequestScreen) optionLabelValue(kind string) (string, string) {
	switch kind {
	case "server":
		if jr.result.MediaType == "movie" {
			if srv := jr.activeRadarr(); srv != nil {
				return "Server", srv.Name
			}
		} else {
			if srv := jr.activeSonarr(); srv != nil {
				return "Server", srv.Name
			}
		}
		return "Server", "—"
	case "profile":
		if jr.result.MediaType == "movie" {
			if srv := jr.activeRadarr(); srv != nil && jr.selectedProfile < len(srv.Profiles) {
				return "Quality", srv.Profiles[jr.selectedProfile].Name
			}
		} else {
			if srv := jr.activeSonarr(); srv != nil && jr.selectedProfile < len(srv.Profiles) {
				return "Quality", srv.Profiles[jr.selectedProfile].Name
			}
		}
		return "Quality", "—"
	case "folder":
		if jr.result.MediaType == "movie" {
			if srv := jr.activeRadarr(); srv != nil && jr.selectedFolder < len(srv.RootFolders) {
				return "Root Folder", srv.RootFolders[jr.selectedFolder].Path
			}
		} else {
			if srv := jr.activeSonarr(); srv != nil && jr.selectedFolder < len(srv.RootFolders) {
				return "Root Folder", srv.RootFolders[jr.selectedFolder].Path
			}
		}
		return "Root Folder", "—"
	case "language":
		if srv := jr.activeSonarr(); srv != nil && jr.selectedLang < len(srv.LanguageProfiles) {
			return "Language", srv.LanguageProfiles[jr.selectedLang].Name
		}
		return "Language", "—"
	case "4k":
		val := "No"
		if jr.is4K {
			val = "Yes"
		}
		return "4K", val
	}
	return "", ""
}
