package app

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/player"
	"github.com/depeter/jellycouch/internal/ui"
)

// Game implements ebiten.Game and manages the overall application.
type Game struct {
	Config  *config.Config
	Client  *jellyfin.Client
	Player  *player.Player
	Cache   *cache.ImageCache
	Screens *ui.ScreenManager

	State         AppState
	Width, Height int

	osd *player.OSDState

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
		osd:     player.NewOSDState(),
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
	g.osd.ShowControlsOverlay()
}

// StopPlayback transitions back to browse mode.
func (g *Game) StopPlayback() {
	if g.Player != nil && g.Player.Playing() {
		itemID := g.Player.ItemID()
		posTicks := int64(g.Player.Position() * 10_000_000)
		g.Player.Stop()
		go g.Client.ReportPlaybackStopped(itemID, posTicks)
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

		wasShowingControls := g.osd.ShowControls
		wasShowingVolume := g.osd.ShowVolume
		g.osd.Update()

		action := player.PollPlayerInput()
		if action == player.ActionStop {
			// Clear OSD before stopping
			g.Player.SetOSDOverlay(1, "")
			g.StopPlayback()
			return nil
		}
		if action != player.ActionNone {
			player.HandleAction(g.Player, action)
			g.osd.ShowControlsOverlay()
			// Show volume overlay on volume/mute actions
			if action == player.ActionVolumeUp || action == player.ActionVolumeDown || action == player.ActionMute {
				g.osd.ShowVolumeOverlay()
			}
		}

		// Update OSD overlays
		if g.osd.ShowControls {
			seekBar := player.FormatSeekBar(g.Player.Position(), g.Player.Duration(), g.Player.Paused())
			if seekBar != "" {
				g.Player.SetOSDOverlay(1, seekBar)
			}
		} else if wasShowingControls {
			g.Player.SetOSDOverlay(1, "")
		}

		if g.osd.ShowVolume {
			vol, _ := g.Player.Volume()
			muted := g.Player.Muted()
			g.Player.SetOSDOverlay(2, player.FormatVolumeOSD(vol, muted))
		} else if wasShowingVolume {
			g.Player.SetOSDOverlay(2, "")
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
