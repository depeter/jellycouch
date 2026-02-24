package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/depeter/jellycouch/assets/fonts"
	"github.com/depeter/jellycouch/internal/app"
	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/ui"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Init fonts
	if err := ui.InitFonts(fonts.LiberationSans); err != nil {
		log.Fatalf("Failed to init fonts: %v", err)
	}

	// Init image cache
	cacheDir := filepath.Join(os.TempDir(), "jellycouch", "images")
	if configDir, err := config.ConfigDir(); err == nil {
		cacheDir = filepath.Join(configDir, "cache", "images")
	}
	imgCache, err := cache.NewImageCache(cacheDir)
	if err != nil {
		log.Fatalf("Failed to init image cache: %v", err)
	}

	// Create game
	var client *jellyfin.Client
	if cfg.Server.URL != "" {
		client = jellyfin.NewClient(cfg.Server.URL)
		if cfg.Server.Token != "" {
			client.SetToken(cfg.Server.Token, cfg.Server.UserID)
		}
	}

	game := app.NewGame(cfg, client, imgCache)

	// Determine initial screen
	if client == nil || cfg.Server.Token == "" {
		// Show login screen
		loginScreen := ui.NewLoginScreen(cfg.Server.URL, func(server, user, pass string) {
			c := jellyfin.NewClient(server)
			if err := c.Authenticate(user, pass); err != nil {
				log.Printf("Auth failed: %v", err)
				return
			}
			// Save credentials
			cfg.Server.URL = server
			cfg.Server.Username = user
			cfg.Server.Token = c.Token()
			cfg.Server.UserID = c.UserID()
			cfg.Save()

			game.Client = c
			pushHomeScreen(game, cfg, imgCache)
		})
		game.Screens.Push(loginScreen)
	} else {
		pushHomeScreen(game, cfg, imgCache)
	}

	// Configure window
	ebiten.SetWindowSize(cfg.UI.Width, cfg.UI.Height)
	ebiten.SetWindowTitle("JellyCouch")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if cfg.UI.Fullscreen {
		ebiten.SetFullscreen(true)
	}

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func pushHomeScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache) {
	home := ui.NewHomeScreen(game.Client, imgCache)
	home.OnItemSelected = func(item jellyfin.MediaItem) {
		pushDetailScreen(game, cfg, imgCache, item)
	}
	home.OnSearch = func() {
		pushSearchScreen(game, cfg, imgCache)
	}
	game.Screens.Replace(home)
}

func pushDetailScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache, item jellyfin.MediaItem) {
	detail := ui.NewDetailScreen(game.Client, imgCache, item)
	detail.OnPlay = func(itemID string, resumeTicks int64) {
		game.StartPlayback(itemID, resumeTicks)
	}
	detail.OnLibrary = func(parentID, title string) {
		lib := ui.NewLibraryScreen(game.Client, imgCache, parentID, title, nil)
		lib.OnItemSelected = func(subItem jellyfin.MediaItem) {
			pushDetailScreen(game, cfg, imgCache, subItem)
		}
		game.Screens.Push(lib)
	}
	game.Screens.Push(detail)
}

func pushSearchScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache) {
	search := ui.NewSearchScreen(game.Client, imgCache)
	search.OnItemSelected = func(item jellyfin.MediaItem) {
		pushDetailScreen(game, cfg, imgCache, item)
	}
	game.Screens.Push(search)
}
