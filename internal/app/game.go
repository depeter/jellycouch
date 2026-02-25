package app

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/jellyseerr"
	"github.com/depeter/jellycouch/internal/player" // used for player.New, player.GetWindowHandle
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
func (g *Game) StartPlayback(itemID string, resumeTicks int64) {
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
	if err := g.Player.LoadFile(streamURL, itemID); err != nil {
		log.Printf("Failed to load file: %v", err)
		return
	}

	// Resume from position if applicable
	if resumeTicks > 0 {
		seconds := float64(resumeTicks) / 10_000_000
		g.Player.SeekAbsolute(seconds)
	}

	// Report playback start
	go g.Client.ReportPlaybackStart(itemID, resumeTicks)

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

	if err := g.Player.LoadFile(url, ""); err != nil {
		log.Printf("Failed to load URL: %v", err)
		return
	}

	g.State = StatePlay
	g.playbackEnded = false
}

// StopPlayback transitions back to browse mode.
func (g *Game) StopPlayback() {
	if g.Player != nil && g.Player.Playing() {
		itemID := g.Player.ItemID()
		posTicks := int64(g.Player.Position() * 10_000_000)
		g.Player.Stop()
		if itemID != "" {
			go g.Client.ReportPlaybackStopped(itemID, posTicks)
		}
	}
	g.State = StateBrowse
}

func (g *Game) Update() error {
	// Alt+Enter toggles fullscreen (works in all modes)
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && ebiten.IsKeyPressed(ebiten.KeyAlt) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	switch g.State {
	case StateBrowse:
		if err := g.Screens.Update(); err != nil {
			return err
		}

	case StatePlay:
		if g.playbackEnded {
			g.State = StateBrowse
			g.playbackEnded = false
			return nil
		}

		// Only handle Escape/Backspace to exit playback — mpv handles everything else
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			g.StopPlayback()
			return nil
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

	case StatePlay:
		// In play mode, mpv owns the window surface via --wid.
		// We don't draw anything — mpv renders directly.
		// However, if we ever need Go-side OSD, we could draw here.
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.Width, g.Height
}
