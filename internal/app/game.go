package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/jellyseerr"
	"github.com/depeter/jellycouch/internal/player"
	"github.com/depeter/jellycouch/internal/ui"
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

	// Set to true when mpv playback ends and we need to return to browse mode
	playbackEnded bool

	overlay        *player.PlaybackOverlay
	currentItem    *jellyfin.MediaItem
	nextEpCh       chan *jellyfin.MediaItem
	nextEpItem     *jellyfin.MediaItem // pre-fetched next episode for direct playback
	nextEpBGRAPath string              // temp file for thumbnail overlay
}

// NewGame creates the Game with all dependencies.
func NewGame(cfg *config.Config, client *jellyfin.Client, imgCache *cache.ImageCache) *Game {
	g := &Game{
		Config:  cfg,
		Client:  client,
		Cache:   imgCache,
		Screens: ui.NewScreenManager(),
		State:   StateBrowse,
		Width:   cfg.UI.Width,
		Height:  cfg.UI.Height,
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
		g.playbackEnded = true
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
		startSec = float64(resumeTicks) / 10_000_000
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
	g.playbackEnded = false
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
	g.playbackEnded = false
}

// StopPlayback transitions back to browse mode.
func (g *Game) StopPlayback() {
	if g.overlay != nil {
		g.overlay.Hide()
		g.overlay = nil
	}
	if g.Player != nil && g.Player.Playing() {
		itemID := g.Player.ItemID()
		posTicks := int64(g.Player.Position() * 10_000_000)
		g.Player.Stop()
		if itemID != "" {
			go g.Client.ReportPlaybackStopped(itemID, posTicks)
		}
	}
	// Drain next-episode channel and clear state
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
}

// prefetchNextEpisode looks up the next episode and pre-fetches its metadata
// and thumbnail for the overlay tooltip. Runs as a goroutine.
func (g *Game) prefetchNextEpisode(item *jellyfin.MediaItem) {
	if item == nil || item.Type != "Episode" || item.SeriesID == "" {
		return
	}

	next := g.lookupNextEpisode(item)
	if next == nil {
		if g.overlay != nil {
			g.overlay.SetNoNextEpisode()
		}
		return
	}

	// Fetch full item details
	full, err := g.Client.GetItem(next.ID)
	if err != nil {
		log.Printf("Failed to fetch next episode: %v", err)
		if g.overlay != nil {
			g.overlay.SetNoNextEpisode()
		}
		return
	}

	g.nextEpItem = full

	info := &player.NextEpisodeInfo{
		Title:         full.Name,
		SeasonNumber:  full.ParentIndexNumber,
		EpisodeNumber: full.IndexNumber,
		ItemID:        full.ID,
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
				g.nextEpBGRAPath = bgraPath
			} else {
				log.Printf("Failed to prepare overlay image: %v", err)
			}
		} else {
			log.Printf("Failed to load next episode thumbnail: %v", err)
		}
	}

	if g.overlay != nil {
		g.overlay.SetNextEpisode(info)
		g.overlay.SetNextUp(full.Name, full.IndexNumber)
	}
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
		if g.playbackEnded {
			g.playbackEnded = false
			if g.nextEpItem != nil {
				next := g.nextEpItem
				g.StopPlayback()
				g.StartPlayback(next.ID, 0, next)
				return nil
			}
			g.State = StateBrowse
			return nil
		}

		// Update overlay auto-hide timer and next-up trigger
		if g.overlay != nil {
			g.overlay.Update()
		}

		// Check for pre-fetched next-episode result
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
	enterPressed := inpututil.IsKeyJustPressed(ebiten.KeyEnter) && !ebiten.IsKeyPressed(ebiten.KeyAlt)

	// === Track select mode (modal — blocks everything) ===
	if g.overlay != nil && g.overlay.Mode == player.OverlayTrackSelect {
		g.overlay.HandleTrackInput(dir, enterPressed, false)
		return
	}

	// === Next-up banner mode ===
	if g.overlay != nil && g.overlay.Mode == player.OverlayNextUp {
		if enterPressed && g.overlay.OnStartNextUp != nil {
			g.overlay.OnStartNextUp()
			return
		}
		// I key or directional input — show control bar
		if inpututil.IsKeyJustPressed(ebiten.KeyI) || dir != player.DirNone {
			g.overlay.Show()
			return
		}
	}

	// === Control bar visible mode ===
	if g.overlay != nil && g.overlay.Mode == player.OverlayBar {
		// Arrow keys and Enter go to bar navigation
		if dir != player.DirNone || enterPressed {
			g.overlay.HandleBarInput(dir, enterPressed, false)
		}

		// Play/pause with Space
		if keyJustPressed(kb.PlayPause) {
			g.Player.TogglePause()
			g.overlay.Show() // re-render and reset timer
		}

		// Volume controls still work while bar is visible
		if keyJustPressed(kb.VolumeUp) {
			g.Player.AdjustVolume(5)
			g.overlay.Show()
		}
		if keyJustPressed(kb.VolumeDown) {
			g.Player.AdjustVolume(-5)
			g.overlay.Show()
		}
		if keyJustPressed(kb.Mute) {
			g.Player.ToggleMute()
			g.overlay.Show()
		}

		// S/A keys open track panels directly
		if keyJustPressed(kb.SubCycle) {
			g.overlay.OpenTrackPanel(player.TrackSub)
		}
		if keyJustPressed(kb.AudioCycle) {
			g.overlay.OpenTrackPanel(player.TrackAudio)
		}

		if keyJustPressed(kb.Fullscreen) {
			ebiten.SetFullscreen(!ebiten.IsFullscreen())
		}

		// I key — just reset the timer
		if inpututil.IsKeyJustPressed(ebiten.KeyI) {
			g.overlay.Show()
		}

		// Mouse input (same as hidden mode)
		g.handlePlaybackMouse()
		return
	}

	// === Hidden mode — seek/volume without showing full overlay ===

	if keyJustPressed(kb.PlayPause) {
		g.Player.TogglePause()
		g.overlay.Show()
	}
	if keyJustPressed(kb.SeekForward) {
		g.Player.Seek(10)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SeekBackward) {
		g.Player.Seek(-10)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SeekForwardLarge) {
		g.Player.Seek(60)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SeekBackwardLarge) {
		g.Player.Seek(-60)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.VolumeUp) {
		g.Player.AdjustVolume(5)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.VolumeDown) {
		g.Player.AdjustVolume(-5)
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.Mute) {
		g.Player.ToggleMute()
		g.Player.ShowProgress()
	}
	if keyJustPressed(kb.SubCycle) {
		g.overlay.Show()
		g.overlay.OpenTrackPanel(player.TrackSub)
	}
	if keyJustPressed(kb.AudioCycle) {
		g.overlay.Show()
		g.overlay.OpenTrackPanel(player.TrackAudio)
	}
	if keyJustPressed(kb.Fullscreen) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	// Enter/OK — show OSD (in hidden mode, Enter shows bar rather than pause)
	if enterPressed {
		g.overlay.Show()
	}

	// I key — show info overlay
	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.overlay.Show()
	}

	g.handlePlaybackMouse()
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
		g.Player.AdjustVolume(5)
		if g.overlay != nil {
			g.overlay.Show()
		}
	} else if scrollY < 0 {
		g.Player.AdjustVolume(-5)
		if g.overlay != nil {
			g.overlay.Show()
		}
	}
}

// parseKey converts a config key name to an ebiten.Key.
func parseKey(name string) (ebiten.Key, bool) {
	switch strings.ToLower(name) {
	case "space":
		return ebiten.KeySpace, true
	case "enter", "return":
		return ebiten.KeyEnter, true
	case "tab":
		return ebiten.KeyTab, true
	case "left":
		return ebiten.KeyArrowLeft, true
	case "right":
		return ebiten.KeyArrowRight, true
	case "up":
		return ebiten.KeyArrowUp, true
	case "down":
		return ebiten.KeyArrowDown, true
	case "a":
		return ebiten.KeyA, true
	case "b":
		return ebiten.KeyB, true
	case "c":
		return ebiten.KeyC, true
	case "d":
		return ebiten.KeyD, true
	case "e":
		return ebiten.KeyE, true
	case "f":
		return ebiten.KeyF, true
	case "g":
		return ebiten.KeyG, true
	case "h":
		return ebiten.KeyH, true
	case "i":
		return ebiten.KeyI, true
	case "j":
		return ebiten.KeyJ, true
	case "k":
		return ebiten.KeyK, true
	case "l":
		return ebiten.KeyL, true
	case "m":
		return ebiten.KeyM, true
	case "n":
		return ebiten.KeyN, true
	case "o":
		return ebiten.KeyO, true
	case "p":
		return ebiten.KeyP, true
	case "q":
		return ebiten.KeyQ, true
	case "r":
		return ebiten.KeyR, true
	case "s":
		return ebiten.KeyS, true
	case "t":
		return ebiten.KeyT, true
	case "u":
		return ebiten.KeyU, true
	case "v":
		return ebiten.KeyV, true
	case "w":
		return ebiten.KeyW, true
	case "x":
		return ebiten.KeyX, true
	case "y":
		return ebiten.KeyY, true
	case "z":
		return ebiten.KeyZ, true
	case "0":
		return ebiten.KeyDigit0, true
	case "1":
		return ebiten.KeyDigit1, true
	case "2":
		return ebiten.KeyDigit2, true
	case "3":
		return ebiten.KeyDigit3, true
	case "4":
		return ebiten.KeyDigit4, true
	case "5":
		return ebiten.KeyDigit5, true
	case "6":
		return ebiten.KeyDigit6, true
	case "7":
		return ebiten.KeyDigit7, true
	case "8":
		return ebiten.KeyDigit8, true
	case "9":
		return ebiten.KeyDigit9, true
	default:
		return 0, false
	}
}

// keyJustPressed checks if the key named by the config string was just pressed.
func keyJustPressed(name string) bool {
	if k, ok := parseKey(name); ok {
		return inpututil.IsKeyJustPressed(k)
	}
	return false
}
