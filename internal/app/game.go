package app

import (
	"fmt"
	"log/slog"
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
	"github.com/depeter/jellycouch/internal/keymap"
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

	// QuitRequested is set to true when the user clicks the Quit button.
	QuitRequested bool

	// Set to true when mpv playback ends and we need to return to browse mode
	playbackEnded atomic.Bool

	overlay        *player.PlaybackOverlay
	currentItem    *jellyfin.MediaItem
	nextEpCh       chan *jellyfin.MediaItem
	nextEpItem     *jellyfin.MediaItem // pre-fetched next episode for direct playback
	nextEpBGRAPath string              // temp file for thumbnail overlay
	prefetchDone   chan *prefetchResult // goroutine sends result here for main-thread application

	// Cursor auto-hide (applies to browse + playback)
	lastMouseX, lastMouseY int
	cursorIdleFrames       int
	cursorHidden           bool

	// True when playback was paused automatically on window focus loss,
	// so we know to resume on refocus without overriding a user-initiated pause.
	pausedByFocus bool

	webCmd    *exec.Cmd
	webExited chan struct{}

	// Keybindings resolved from config at NewGame time.
	Keybinds keymap.Keybindings
}

// RunOnMain schedules fn to execute on the main (Ebitengine) thread at the
// start of the next Update tick. Safe to call from any goroutine.
// Prefer this over direct channel sends so call sites stay readable and
// future changes (e.g. capacity tuning or typed events) have a single
// chokepoint.
func (g *Game) RunOnMain(fn func()) {
	g.MainActions <- fn
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
		Config:      cfg,
		Client:      client,
		Cache:       imgCache,
		Screens:     ui.NewScreenManager(),
		State:       StateBrowse,
		Width:       1920,
		Height:      1080,
		MainActions: make(chan func(), 16),
		Keybinds:    keymap.Resolve(cfg.Keybindings),
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
			slog.Error("init player", "err", err)
			return
		}
	}

	// Get window handle and set on mpv
	wid, err := player.GetWindowHandle()
	if err != nil {
		slog.Error("get window handle", "err", err)
		return
	}
	if err := g.Player.SetWindowID(wid); err != nil {
		slog.Error("set window ID", "err", err)
	}

	streamURL := g.Client.GetStreamURL(itemID)
	var startSec float64
	if resumeTicks > 0 {
		startSec = float64(resumeTicks) / constants.TicksPerSecond
	}
	if err := g.Player.LoadFile(streamURL, itemID, startSec); err != nil {
		slog.Error("load file", "err", err)
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

	g.setState(StatePlay)
	g.playbackEnded.Store(false)
}

// PlayURL plays an arbitrary URL (e.g. YouTube trailer) via mpv without Jellyfin progress reporting.
func (g *Game) PlayURL(url string) {
	if g.Player == nil {
		if err := g.InitPlayer(); err != nil {
			slog.Error("init player", "err", err)
			return
		}
	}

	wid, err := player.GetWindowHandle()
	if err != nil {
		slog.Error("get window handle", "err", err)
		return
	}
	if err := g.Player.SetWindowID(wid); err != nil {
		slog.Error("set window ID", "err", err)
	}

	slog.Info("PlayURL loading", "url", url)
	if err := g.Player.LoadFile(url, "", 0); err != nil {
		slog.Error("load URL", "err", err)
		return
	}

	g.currentItem = nil
	g.nextEpCh = make(chan *jellyfin.MediaItem, 1)

	g.overlay = player.NewPlaybackOverlay(g.Player, g.Width, g.Height)
	g.overlay.OnStop = func() { g.StopPlayback() }
	g.overlay.Show()

	g.setState(StatePlay)
	g.playbackEnded.Store(false)
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
	g.pausedByFocus = false
	g.setState(StateBrowse)
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
		slog.Warn("fetch next episode", "err", err)
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
				slog.Warn("prepare overlay image", "err", err)
			}
		} else {
			slog.Warn("load next episode thumbnail", "err", err)
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
		slog.Error("start web app", "err", err)
		return
	}
	g.webCmd = cmd
	g.webExited = make(chan struct{})
	g.setState(StateWeb)

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
	g.setState(StateBrowse)
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
			slog.Warn("fetch next episode", "err", err)
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

// updateCursorAutoHide hides the cursor after ~3s of no mouse movement and
// restores it the moment the user moves the mouse. Applies to all UI states.
func (g *Game) updateCursorAutoHide() {
	const cursorHideDelay = 180 // ~3s at 60fps
	mx, my := ebiten.CursorPosition()
	if mx != g.lastMouseX || my != g.lastMouseY {
		g.lastMouseX, g.lastMouseY = mx, my
		g.cursorIdleFrames = 0
		if g.cursorHidden {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
			g.cursorHidden = false
		}
		return
	}
	g.cursorIdleFrames++
	if !g.cursorHidden && g.cursorIdleFrames >= cursorHideDelay {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
		g.cursorHidden = true
	}
}

// updateFocusPause pauses playback when the window loses focus and resumes
// when it regains focus, but only if we were the one who paused (so a
// user-initiated pause is not overridden).
func (g *Game) updateFocusPause() {
	if g.Player == nil {
		return
	}
	focused := ebiten.IsFocused()
	if !focused && g.Player.Playing() && !g.Player.Paused() {
		if err := g.Player.TogglePause(); err == nil {
			g.pausedByFocus = true
		}
		return
	}
	if focused && g.pausedByFocus {
		if err := g.Player.TogglePause(); err == nil {
			g.pausedByFocus = false
		}
	}
}

func (g *Game) Update() error {
	if g.QuitRequested {
		return ebiten.Termination
	}

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

	// F12 toggles debug overlay (works in all modes)
	ui.ToggleDebugOverlay()

	g.updateCursorAutoHide()

	switch g.State {
	case StateBrowse:
		if err := g.Screens.Update(); err != nil {
			return err
		}

	case StatePlay:
		g.updateFocusPause()

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
			g.setState(StateBrowse)
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
	// Preserve logical 1080 height (keeping text/posters readable on 4K via
	// ebiten's upscaling) but grow the logical width so ultrawide displays
	// use the extra horizontal room instead of being letterboxed. UIScale
	// shrinks the logical canvas by the scale factor so Ebitengine upscales
	// more — text/posters keep their constant pixel sizes but appear
	// physically larger on screen.
	scale := g.Config.Display.UIScale
	if scale < 0.5 {
		scale = 1.0
	}
	logicalH := int(float64(ui.LogicalScreenHeight) / scale)
	minW := int(float64(ui.LogicalScreenWidth) / scale)
	w, h := outsideWidth, outsideHeight
	if h <= 0 || w <= 0 {
		return minW, logicalH
	}
	logicalW := w * logicalH / h
	if logicalW < minW {
		// Narrower than 16:9 — letterbox rather than stretch layout below
		// the design width.
		logicalW = minW
	}
	g.Width, g.Height = logicalW, logicalH
	ui.SetScreenSize(logicalW, logicalH)
	return logicalW, logicalH
}

// handlePlaybackInput forwards keybinds, media keys, and mouse input to mpv.
// Input routing depends on the overlay state: hidden, bar visible, or track select.
func (g *Game) handlePlaybackInput() {
	if g.Player == nil {
		return
	}

	// Determine directional input
	dir := player.DirNone
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		dir = player.DirRight
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		dir = player.DirLeft
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		dir = player.DirUp
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
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
		g.handleInputBar(dir, enterPressed)
	default:
		g.handleInputHidden(enterPressed)
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
	if keybindJustPressed(g.Keybinds.ShowInfo) || dir != player.DirNone {
		g.overlay.Show()
	}
}

// handleInputBar handles input when the control bar is visible.
func (g *Game) handleInputBar(dir player.Direction, enter bool) {
	if dir != player.DirNone || enter {
		g.overlay.HandleBarInput(dir, enter, false)
	}
	if g.overlay == nil {
		return
	}
	g.handleCommonPlaybackKeys(true)
	if keybindJustPressed(g.Keybinds.ShowInfo) {
		g.overlay.Show()
	}
	g.handlePlaybackMouse()
}

// keybindJustPressed returns true if k is bound and was just pressed.
// Safe to call with the Unbound sentinel — returns false instead of
// matching ebiten.KeyA (which has value 0).
func keybindJustPressed(k ebiten.Key) bool {
	return keymap.IsBound(k) && inpututil.IsKeyJustPressed(k)
}

// handleInputHidden handles input when the overlay is hidden.
func (g *Game) handleInputHidden(enter bool) {
	kb := g.Keybinds
	if keybindJustPressed(kb.PlayPause) {
		g.Player.TogglePause()
		g.overlay.Show()
	}
	if keybindJustPressed(kb.SeekSmallForward) {
		g.Player.Seek(player.SeekSmall)
		g.Player.ShowProgress()
	}
	if keybindJustPressed(kb.SeekSmallBack) {
		g.Player.Seek(-player.SeekSmall)
		g.Player.ShowProgress()
	}
	if keybindJustPressed(kb.SeekLargeForward) {
		g.Player.Seek(player.SeekLarge)
		g.Player.ShowProgress()
	}
	if keybindJustPressed(kb.SeekLargeBack) {
		g.Player.Seek(-player.SeekLarge)
		g.Player.ShowProgress()
	}
	g.handleCommonPlaybackKeys(false)
	if enter {
		g.overlay.Show()
	}
	if keybindJustPressed(kb.ShowInfo) {
		g.overlay.Show()
	}
	g.handlePlaybackMouse()
}

// handleCommonPlaybackKeys handles volume, track, and fullscreen keys shared
// between bar-visible and hidden modes. When barVisible is true, actions
// re-show the overlay bar; otherwise they show a brief progress indicator.
func (g *Game) handleCommonPlaybackKeys(barVisible bool) {
	kb := g.Keybinds
	show := func() {
		if barVisible {
			g.overlay.Show()
		} else {
			g.Player.ShowProgress()
		}
	}
	if keybindJustPressed(kb.VolumeUp) {
		g.Player.AdjustVolume(player.VolumeStep)
		show()
	}
	if keybindJustPressed(kb.VolumeDown) {
		g.Player.AdjustVolume(-player.VolumeStep)
		show()
	}
	if keybindJustPressed(kb.Mute) {
		g.Player.ToggleMute()
		show()
	}
	if keybindJustPressed(kb.CycleSubtitles) && g.overlay != nil {
		if !barVisible {
			g.overlay.Show()
		}
		g.overlay.OpenTrackPanel(player.TrackSub)
	}
	if keybindJustPressed(kb.CycleAudio) && g.overlay != nil {
		if !barVisible {
			g.overlay.Show()
		}
		g.overlay.OpenTrackPanel(player.TrackAudio)
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

