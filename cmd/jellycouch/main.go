package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/depeter/jellycouch/assets/fonts"
	"github.com/depeter/jellycouch/assets/icon"
	"github.com/depeter/jellycouch/internal/app"
	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/jellyseerr"
	"github.com/depeter/jellycouch/internal/ui"
	"github.com/depeter/jellycouch/internal/webview"
)

func main() {
	// Webview child process mode — must be checked before any Ebitengine init
	if len(os.Args) >= 3 && os.Args[1] == "--webview" {
		webview.RunWebview(os.Args[2])
		return
	}

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

	// Init Jellyseerr client if configured
	if cfg.Jellyseerr.URL != "" && cfg.Jellyseerr.APIKey != "" {
		game.Jellyseerr = jellyseerr.NewClient(cfg.Jellyseerr.URL, cfg.Jellyseerr.APIKey)
	}

	sf := &screenFactory{game: game, cfg: cfg, imgCache: imgCache}

	// Create and wire the global navbar
	navbar := ui.NewNavBar()
	navbar.OnNavigate = func(action, id, title string) {
		switch action {
		case "home":
			game.Screens.ClearStack()
			sf.pushHome()
		case "library":
			game.Screens.ClearStack()
			sf.pushHome()
			sf.pushLibrary(id, title, nil)
		case "discovery":
			game.Screens.ClearStack()
			sf.pushHome()
			sf.pushJellyseerrDiscover()
		case "apps":
			game.Screens.ClearStack()
			sf.pushHome()
			sf.pushWebApps()
		case "settings":
			game.Screens.ClearStack()
			sf.pushHome()
			sf.pushSettings()
		case "quit":
			game.QuitRequested = true
		}
	}
	navbar.OnSearch = func(query string) {
		game.Screens.ClearStack()
		sf.pushHome()
		sf.pushSearch(query)
	}
	navbar.JellyseerrEnabled = func() bool {
		return game.Jellyseerr != nil
	}
	navbar.WebAppsEnabled = func() bool {
		return true
	}
	game.Screens.NavBar = navbar

	// Determine initial screen
	if client == nil || cfg.Server.Token == "" {
		sf.pushLogin(navbar)
	} else {
		// Skip synchronous token validation — the home screen validates
		// asynchronously and redirects to login on auth errors (401).
		// This avoids blocking startup on slow/unreachable servers.
		sf.pushHome()
		sf.loadNavBarViews()
	}

	// Configure window
	ebiten.SetWindowSize(1920, 1080)
	ebiten.SetWindowTitle("JellyCouch")
	ebiten.SetWindowIcon(icon.Generate())
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
	// Clean up resources on exit
	if game.State == app.StateWeb {
		game.StopWebApp()
	}
	if game.Player != nil {
		game.Player.Destroy()
	}
}
