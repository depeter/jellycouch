package app

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/constants"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/jellyseerr"
	"github.com/depeter/jellycouch/internal/player"
	"github.com/depeter/jellycouch/internal/ui"
	"github.com/depeter/jellycouch/internal/webview"
)

// Game implements ebiten.Game and manages the overall application.
type Game struct {
	Config     *config.Config
	Client     *jellyfin.Client
	Jellyseerr *jellyseerr.Client
	Player     *player.Player
	Cache      *cache.ImageCache
	Screens    *ui.ScreenManager

	State         AppState
	Width, Height int

	// MainActions receives functions to execute on the main Ebitengine thread.
	// Background goroutines must send closures here instead of directly mutating
	// game state, to avoid data races with Update()/Draw().
	MainActions chan func()

	// Set to true when mpv playback ends and we need to return to browse mode
	playbackEnded atomic.Bool

	overlay        *player.PlaybackOverlay
	currentItem    *jellyfin.MediaItem
	nextEpCh       chan *jellyfin.MediaItem
	nextEpItem     *jellyfin.MediaItem // pre-fetched next episode for direct playback
	nextEpBGRAPath string              // temp file for thumbnail overlay
	prefetchDone   chan *prefetchResult // goroutine sends result here for main-thread application

	startFullscreen bool // unused, kept for config compatibility

	// Cursor auto-hide during playback
	lastMouseX, lastMouseY int
	cursorIdleFrames       int
	cursorHidden           bool

	webCmd    *exec.Cmd
	webExited chan struct{}
}

// prefetchResult carries the outcome of prefetchNextEpisode back to the main thread.
type prefetchResult struct {
	item     *jellyfin.MediaItem
	bgraPath string
	info     *player.NextEpisodeInfo // nil means no next episode found
}

// NewGame creates the Game with all dependencies.
func NewGame(cfg *config.Config, client *jellyfin.Client, imgCache *cache.ImageCache) *Game {
	g := &Game{
		Config:          cfg,
		Client:          client,
		Cache:           imgCache,
		Screens:         ui.NewScreenManager(),
		State:           StateBrowse,
		Width:           cfg.UI.Width,
		Height:          cfg.UI.Height,
		startFullscreen: cfg.UI.Fullscreen,
		MainActions:     make(chan func(), 16),
	}
	return g
}

// InitPlayer creates the mpv player instance. Call after the window is visible.
func (g *Game) InitPlayer() error {
	p, err := player.New(g.Config)
	if err != nil {
		return err
	}
	p.OnPlaybackEnd = func() {
		g.playbackEnded.Store(true)
	}
	g.Player = p
	return nil
}

// StartPlayback transitions to play mode.
func (g *Game) StartPlayback(itemID string, resumeTicks int64, item *jellyfin.MediaItem) {
	if g.Player == nil {
		if err := g.InitPlayer(); err != nil {
			log.Printf("Failed to init player: %v", err)
			return
		}
	}

	// Get window handle and set on mpv
	wid, err := player.GetWindowHandle()
	if err != nil {
		log.Printf("Failed to get window handle: %v", err)
		return
	}
	if err := g.Player.SetWindowID(wid); err != nil {
		log.Printf("Failed to set window ID: %v", err)
	}

	streamURL := g.Client.GetStreamURL(itemID)
	var startSec float64
	if resumeTicks > 0 {
		startSec = float64(resumeTicks) / constants.TicksPerSecond
	}
	if err := g.Player.LoadFile(streamURL, itemID, startSec); err != nil {
		log.Printf("Failed to load file: %v", err)
		return
	}

	// Report playback start
	go g.Client.ReportPlaybackStart(itemID, resumeTicks)

	g.currentItem = item
	g.nextEpCh = make(chan *jellyfin.MediaItem, 1)
	g.nextEpItem = nil
	g.nextEpBGRAPath = ""
	g.prefetchDone = make(chan *prefetchResult, 1)

	g.overlay = player.NewPlaybackOverlay(g.Player, g.Width, g.Height)
	g.overlay.OnStop = func() { g.StopPlayback() }
	if item != nil && item.Type == "Episode" {
		g.overlay.SetShowNextButton(true)
		g.overlay.OnNextEpisode = func() { g.playNextEpisode() }
		g.overlay.OnStartNextUp = func() { g.playNextEpisode() }
		go g.prefetchNextEpisode(item)
	}
	g.overlay.Show()

	g.State = StatePlay
	g.playbackEnded.Store(false)
	g.cursorIdleFrames = 0
	g.cursorHidden = false
}

// PlayURL plays an arbitrary URL (e.g. YouTube trailer) via mpv without Jellyfin progress reporting.
func (g *Game) PlayURL(url string) {
	if g.Player == nil {
		if err := g.InitPlayer(); err != nil {
			log.Printf("Failed to init player: %v", err)
			return
		}
	}

	wid, err := player.GetWindowHandle()
	if err != nil {
		log.Printf("Failed to get window handle: %v", err)
		return
	}
	if err := g.Player.SetWindowID(wid); err != nil {
		log.Printf("Failed to set window ID: %v", err)
	}

	log.Printf("PlayURL: loading %s", url)
	if err := g.Player.LoadFile(url, "", 0); err != nil {
		log.Printf("Failed to load URL: %v", err)
		return
	}

	g.currentItem = nil
	g.nextEpCh = make(chan *jellyfin.MediaItem, 1)

	g.overlay = player.NewPlaybackOverlay(g.Player, g.Width, g.Height)
	g.overlay.OnStop = func() { g.StopPlayback() }
	g.overlay.Show()

	g.State = StatePlay
	g.playbackEnded.Store(false)
	g.cursorIdleFrames = 0
	g.cursorHidden = false
}

// StopPlayback transitions back to browse mode.
func (g *Game) StopPlayback() {
	if g.overlay != nil {
		o := g.overlay
		g.overlay = nil
		o.Cleanup()
	}
	if g.Player != nil {
		itemID := g.Player.ItemID()
		posTicks := int64(g.Player.Position() * constants.TicksPerSecond)
		if g.Player.Playing() {
			g.Player.Stop()
		}
		if itemID != "" {
			go g.Client.ReportPlaybackStopped(itemID, posTicks)
		}
	}
	// Drain next-episode channels and clear state
	if g.prefetchDone != nil {
		select {
		case <-g.prefetchDone:
		default:
		}
		g.prefetchDone = nil
	}
	if g.nextEpCh != nil {
		select {
		case <-g.nextEpCh:
		default:
		}
	}
	if g.nextEpBGRAPath != "" {
		os.Remove(g.nextEpBGRAPath)
		g.nextEpBGRAPath = ""
	}
	g.nextEpItem = nil
	g.currentItem = nil
	g.State = StateBrowse
	if g.cursorHidden {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		g.cursorHidden = false
	}
}

// prefetchNextEpisode looks up the next episode and pre-fetches its metadata
// and thumbnail for the overlay tooltip. Runs as a goroutine; sends the result
// on g.prefetchDone so the main thread can safely apply it.
func (g *Game) prefetchNextEpisode(item *jellyfin.MediaItem) {
	ch := g.prefetchDone // capture locally

	if item == nil || item.Type != "Episode" || item.SeriesID == "" {
		ch <- &prefetchResult{} // signal no next episode to unblock main thread
		return
	}

	next := g.lookupNextEpisode(item)
	if next == nil {
		ch <- &prefetchResult{} // info == nil signals no next episode
		return
	}

	// Fetch full item details
	full, err := g.Client.GetItem(next.ID)
	if err != nil {
		log.Printf("Failed to fetch next episode: %v", err)
		ch <- &prefetchResult{}
		return
	}

	info := &player.NextEpisodeInfo{
		Title:         full.Name,
		SeasonNumber:  full.ParentIndexNumber,
		EpisodeNumber: full.IndexNumber,
		ItemID:        full.ID,
	}

	result := &prefetchResult{
		item: full,
		info: info,
	}

	// Try to fetch a thumbnail image
	imgURL := ""
	if _, ok := full.ImageTags["Thumb"]; ok {
		imgURL = g.Client.GetImageURL(full.ID, jellyfin.ImageThumb, 480, 0)
	} else if _, ok := full.ImageTags["Primary"]; ok {
		imgURL = g.Client.GetImageURL(full.ID, jellyfin.ImagePrimary, 480, 0)
	}

	if imgURL != "" {
		img, err := g.Cache.LoadDecodedImage(imgURL)
		if err == nil {
			bgraPath := filepath.Join(g.Cache.CacheDir(), fmt.Sprintf("nextep_%s.bgra", full.ID))
			w, h, err := player.PrepareOverlayImage(img, bgraPath)
			if err == nil {
				info.ImagePath = bgraPath
				info.ImageW = w
				info.ImageH = h
				result.bgraPath = bgraPath
			} else {
				log.Printf("Failed to prepare overlay image: %v", err)
			}
		} else {
			log.Printf("Failed to load next episode thumbnail: %v", err)
		}
	}

	ch <- result
}

// playNextEpisode plays the pre-fetched next episode directly, or falls back
// to the async lookup flow.
func (g *Game) playNextEpisode() {
	if g.nextEpItem != nil {
		item := g.nextEpItem
		g.StopPlayback()
		g.StartPlayback(item.ID, 0, item)
		return
	}
	// Fallback: trigger async lookup
	g.findAndQueueNextEpisode()
}

// StartWebApp launches a web app in a child webview process.
func (g *Game) StartWebApp(url string) {
	cmd, err := webview.StartWebApp(url)
	if err != nil {
		log.Printf("Failed to start web app: %v", err)
		return
	}
	g.webCmd = cmd
	g.webExited = make(chan struct{})
	g.State = StateWeb

	go func() {
		cmd.Wait()
		close(g.webExited)
	}()
}

// StopWebApp kills the webview child process and returns to browse mode.
func (g *Game) StopWebApp() {
	if g.webCmd != nil && g.webCmd.Process != nil {
		// Check if already exited before killing
		select {
		case <-g.webExited:
			// Already exited, no need to kill
		default:
			g.webCmd.Process.Kill()
			<-g.webExited
		}
	}
	g.webCmd = nil
	g.webExited = nil
	g.State = StateBrowse
}

// findAndQueueNextEpisode looks up the next episode and sends it on nextEpCh.
func (g *Game) findAndQueueNextEpisode() {
	item := g.currentItem
	if item == nil || item.Type != "Episode" || item.SeriesID == "" {
		return
	}
	ch := g.nextEpCh
	go func() {
		next := g.lookupNextEpisode(item)
		if next == nil {
			ch <- nil
			return
		}
		full, err := g.Client.GetItem(next.ID)
		if err != nil {
			log.Printf("Failed to fetch next episode: %v", err)
			ch <- nil
			return
		}
		ch <- full
	}()
}

// lookupNextEpisode finds the next episode after the given one.
func (g *Game) lookupNextEpisode(item *jellyfin.MediaItem) *jellyfin.MediaItem {
	// Try next episode in the same season
	if item.SeasonID != "" {
		episodes, err := g.Client.GetEpisodes(item.SeriesID, item.SeasonID)
		if err == nil {
			for i, ep := range episodes {
				if ep.ID == item.ID && i+1 < len(episodes) {
					return &episodes[i+1]
				}
			}
		}
	}

	// Last episode of the season — try next season
	seasons, err := g.Client.GetSeasons(item.SeriesID)
	if err != nil {
		return nil
	}
	foundSeason := false
	for _, season := range seasons {
		if foundSeason {
			eps, err := g.Client.GetEpisodes(item.SeriesID, season.ID)
			if err == nil && len(eps) > 0 {
				return &eps[0]
			}
		}
		if season.ID == item.SeasonID {
			foundSeason = true
		}
	}
	return nil
}

func (g *Game) Update() error {
	// Drain main-thread action queue (closures from background goroutines)
	for {
		select {
		case fn := <-g.MainActions:
			fn()
		default:
			goto actionsDrained
		}
	}
actionsDrained:

	// Alt+Enter toggles fullscreen (works in all modes)
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && ebiten.IsKeyPressed(ebiten.KeyAlt) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	// F12 toggles debug overlay (works in all modes)
	ui.ToggleDebugOverlay()

	switch g.State {
	case StateBrowse:
		if err := g.Screens.Update(); err != nil {
			return err
		}

	case StatePlay:
		if g.playbackEnded.Swap(false) {
			if g.nextEpItem != nil {
				next := g.nextEpItem
				g.StopPlayback()
				g.StartPlayback(next.ID, 0, next)
				return nil
			}
			g.StopPlayback()
			return nil
		}

		// Update overlay auto-hide timer and next-up trigger
		if g.overlay != nil {
			g.overlay.Update()
		}

		// Auto-hide cursor after 3 seconds of no mouse movement
		const cursorHideDelay = 180 // ~3s at 60fps
		mx, my := ebiten.CursorPosition()
		if mx != g.lastMouseX || my != g.lastMouseY {
			g.lastMouseX, g.lastMouseY = mx, my
			g.cursorIdleFrames = 0
			if g.cursorHidden {
				ebiten.SetCursorMode(ebiten.CursorModeVisible)
				g.cursorHidden = false
			}
		} else {
			g.cursorIdleFrames++
			if !g.cursorHidden && g.cursorIdleFrames >= cursorHideDelay {
				ebiten.SetCursorMode(ebiten.CursorModeHidden)
				g.cursorHidden = true
			}
		}

		// Apply prefetched next-episode result from goroutine (race-free)
		if g.prefetchDone != nil {
			select {
			case res := <-g.prefetchDone:
				g.prefetchDone = nil
				if res.info != nil {
					g.nextEpItem = res.item
					g.nextEpBGRAPath = res.bgraPath
					if g.overlay != nil {
						g.overlay.SetNextEpisode(res.info)
						g.overlay.SetNextUp(res.info.Title, res.info.EpisodeNumber)
					}
				} else if g.overlay != nil {
					g.overlay.SetNoNextEpisode()
				}
			default:
			}
		}

		// Check for async next-episode lookup result (fallback path)
		if g.nextEpCh != nil {
			select {
			case nextItem := <-g.nextEpCh:
				g.nextEpCh = nil
				if nextItem != nil {
					if g.nextEpItem != nil {
						// Already have a stored result — this shouldn't happen, ignore
					} else {
						g.nextEpItem = nextItem
						if g.overlay != nil {
							g.overlay.SetNextUp(nextItem.Name, nextItem.IndexNumber)
						}
					}
				} else if g.Player != nil {
					g.Player.ShowText("No next episode", 3000)
				}
			default:
			}
		}

		// Esc/Back — context-dependent behavior
		backPressed := inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
			inpututil.IsKeyJustPressed(ebiten.KeyBackspace) ||
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) ||
			ui.EvdevBackJustPressed()

		if backPressed && g.overlay != nil {
			switch g.overlay.Mode {
			case player.OverlayTrackSelect:
				g.overlay.HandleTrackInput(player.DirNone, false, true)
				return nil
			case player.OverlayBar:
				g.overlay.Hide()
				return nil
			case player.OverlayNextUp:
				// Back on next-up banner — fall through to stop playback
			default:
				// OverlayHidden — fall through to stop playback
			}
		}

		if backPressed {
			g.StopPlayback()
			return nil
		}

		// Forward playback controls to mpv (required on Windows where
		// embedded mpv doesn't receive keyboard input directly)
		g.handlePlaybackInput()

	case StateWeb:
		select {
		case <-g.webExited:
			g.webCmd = nil
			g.webExited = nil
			g.State = StateBrowse
		default:
		}
	}

	ui.UpdateInputState()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	switch g.State {
	case StateBrowse:
		screen.Fill(ui.ColorBackground)
		g.Screens.Draw(screen)
		ui.DrawDebugOverlay(screen)

	case StatePlay:
		// In play mode, mpv owns the window surface via --wid.
		// We don't draw anything — mpv renders directly.
		// During playback, evdev events are still logged to terminal.

	case StateWeb:
		screen.Fill(ui.ColorBackground)
		ui.DrawTextCentered(screen, "Web app running...",
			float64(g.Width)/2, float64(g.Height)/2,
			ui.FontSizeHeading, ui.ColorTextMuted)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.Width, g.Height
}

// handlePlaybackInput forwards keybinds, media keys, and mouse input to mpv.
// Input routing depends on the overlay state: hidden, bar visible, or track select.
func (g *Game) handlePlaybackInput() {
	if g.Player == nil {
		return
	}
	kb := &g.Config.Keybinds

	// Determine directional input
	dir := player.DirNone
	if keyJustPressed(kb.SeekForward) {
		dir = player.DirRight
	} else if keyJustPressed(kb.SeekBackward) {
		dir = player.DirLeft
	} else if keyJustPressed(kb.SeekForwardLarge) {
		dir = player.DirUp
	} else if keyJustPressed(kb.SeekBackwardLarge) {
		dir = player.DirDown
	}
	enterPressed := inpututil.IsKeyJustPressed(ebiten.KeyEnter) && !ui.IsModifierPressed()

	if g.overlay == nil {
		return
	}

	switch g.overlay.Mode {
	case player.OverlayTrackSelect:
		g.handleInputTrackSelect(dir, enterPressed)
	case player.OverlayNextUp:
		g.handleInputNextUp(dir, enterPressed)
	case player.OverlayBar:
		g.handleInputBar(dir, enterPressed, kb)
	default:
		g.handleInputHidden(enterPressed, kb)
	}
}

// handleInputTrackSelect handles input when the track selection modal is open.
func (g *Game) handleInputTrackSelect(dir player.Direction, enter bool) {
	g.overlay.HandleTrackInput(dir, enter, false)
}

// handleInputNextUp handles input when the "Up Next" banner is showing.
func (g *Game) handleInputNextUp(dir player.Direction, enter bool) {
	if enter && g.overlay.OnStartNextUp != nil {
		g.overlay.OnStartNextUp()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyI) || dir != player.DirNone {
		g.overlay.Show()
	}
}

// handleInputBar handles input when the control bar is visible.
func (g *Game) handleInputBar(dir player.Direction, enter bool, kb *config.KeybindConfig) {
	if dir != player.DirNone || enter {
		g.overlay.HandleBarInput(dir, enter, false)
	}
	if g.overlay == nil {
		return
	}
	g.handleCommonPlaybackKeys(kb, true)
	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.overlay.Show()
	}
	g.handlePlaybackMouse()
}

// handleInputHidden handles input when the overlay is hidden.
func (g *Game) handleInputHidden(enter bool, kb *config.KeybindConfig) {
	if keyJustPressed(kb.PlayPause) {
		g.Player.TogglePause()
		g.overlay.Show()
	}
	if keyJustPressed(kb.SeekForward) {
		g.Player.Seek(player.SeekSmall)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SeekBackward) {
		g.Player.Seek(-player.SeekSmall)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SeekForwardLarge) {
		g.Player.Seek(player.SeekLarge)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SeekBackwardLarge) {
		g.Player.Seek(-player.SeekLarge)
		g.Player.ShowProgress()
	}
	g.handleCommonPlaybackKeys(kb, false)
	if enter {
		g.overlay.Show()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.overlay.Show()
	}
	g.handlePlaybackMouse()
}

// handleCommonPlaybackKeys handles volume, track, and fullscreen keys shared
// between bar-visible and hidden modes. When barVisible is true, actions
// re-show the overlay bar; otherwise they show a brief progress indicator.
func (g *Game) handleCommonPlaybackKeys(kb *config.KeybindConfig, barVisible bool) {
	show := func() {
		if barVisible {
			g.overlay.Show()
		} else {
			g.Player.ShowProgress()
		}
	}
	if keyJustPressed(kb.VolumeUp) {
		g.Player.AdjustVolume(player.VolumeStep)
		show()
	}
	if keyJustPressed(kb.VolumeDown) {
		g.Player.AdjustVolume(-player.VolumeStep)
		show()
	}
	if keyJustPressed(kb.Mute) {
		g.Player.ToggleMute()
		show()
	}
	if keyJustPressed(kb.SubCycle) && g.overlay != nil {
		if !barVisible {
			g.overlay.Show()
		}
		g.overlay.OpenTrackPanel(player.TrackSub)
	}
	if keyJustPressed(kb.AudioCycle) && g.overlay != nil {
		if !barVisible {
			g.overlay.Show()
		}
		g.overlay.OpenTrackPanel(player.TrackAudio)
	}
	if keyJustPressed(kb.Fullscreen) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
}

// handlePlaybackMouse handles mouse input during playback (same in all overlay modes).
func (g *Game) handlePlaybackMouse() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.Player.TogglePause()
		if g.overlay != nil {
			g.overlay.Show()
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if g.overlay != nil {
			g.overlay.Show()
		}
	}
	_, scrollY := ebiten.Wheel()
	if scrollY > 0 {
		g.Player.AdjustVolume(player.VolumeStep)
		if g.overlay != nil {
			g.overlay.Show()
		}
	} else if scrollY < 0 {
		g.Player.AdjustVolume(-player.VolumeStep)
		if g.overlay != nil {
			g.overlay.Show()
		}
	}
}

