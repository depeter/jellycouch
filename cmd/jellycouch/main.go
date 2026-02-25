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
	"github.com/depeter/jellycouch/internal/jellyseerr"
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

	// Init Jellyseerr client if configured
	if cfg.Jellyseerr.URL != "" && cfg.Jellyseerr.APIKey != "" {
		game.Jellyseerr = jellyseerr.NewClient(cfg.Jellyseerr.URL, cfg.Jellyseerr.APIKey)
	}

	// Determine initial screen
	if client == nil || cfg.Server.Token == "" {
		// Show login screen
		loginScreen := ui.NewLoginScreen(cfg.Server.URL, func(screen *ui.LoginScreen, server, user, pass string) {
			screen.Busy = true
			screen.Error = ""
			go func() {
				c := jellyfin.NewClient(server)
				if err := c.Authenticate(user, pass); err != nil {
					screen.Error = "Login failed: " + err.Error()
					screen.Busy = false
					return
				}
				// Save credentials
				cfg.Server.URL = server
				cfg.Server.Username = user
				cfg.Server.Token = c.Token()
				cfg.Server.UserID = c.UserID()
				cfg.Save()

				game.Client = c
				screen.Busy = false
				pushHomeScreen(game, cfg, imgCache)
			}()
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
	home.OnSearch = func(query string) {
		pushSearchScreen(game, cfg, imgCache, query)
	}
	home.OnSettings = func() {
		pushSettingsScreen(game, cfg)
	}
	home.OnRequests = func() {
		pushJellyseerrDiscoverScreen(game, cfg, imgCache)
	}
	home.JellyseerrEnabled = func() bool {
		return game.Jellyseerr != nil
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

func pushSearchScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache, query string) {
	search := ui.NewSearchScreen(game.Client, imgCache)
	search.OnItemSelected = func(item jellyfin.MediaItem) {
		pushDetailScreen(game, cfg, imgCache, item)
	}
	if query != "" {
		search.SetInitialQuery(query)
	}
	game.Screens.Push(search)
}

func pushSettingsScreen(game *app.Game, cfg *config.Config) {
	settings := ui.NewSettingsScreen(cfg, func() {
		cfg.Save()
		// Reinitialize Jellyseerr client if config changed
		if cfg.Jellyseerr.URL != "" && cfg.Jellyseerr.APIKey != "" {
			game.Jellyseerr = jellyseerr.NewClient(cfg.Jellyseerr.URL, cfg.Jellyseerr.APIKey)
		} else {
			game.Jellyseerr = nil
		}
	})
	game.Screens.Push(settings)
}

func pushJellyseerrDiscoverScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache) {
	if game.Jellyseerr == nil {
		return
	}
	discover := ui.NewJellyseerrDiscoverScreen(game.Jellyseerr, imgCache)
	discover.OnItemSelected = func(result jellyseerr.SearchResult) {
		pushJellyseerrRequestScreen(game, cfg, imgCache, result)
	}
	discover.OnRequests = func() {
		pushJellyseerrRequestsScreen(game, cfg, imgCache)
	}
	discover.OnSearch = func() {
		pushJellyseerrSearchScreen(game, cfg, imgCache)
	}
	game.Screens.Push(discover)
}

func pushJellyseerrSearchScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache) {
	if game.Jellyseerr == nil {
		return
	}
	search := ui.NewJellyseerrSearchScreen(game.Jellyseerr, imgCache)
	search.OnResultSelected = func(result jellyseerr.SearchResult) {
		pushJellyseerrRequestScreen(game, cfg, imgCache, result)
	}
	game.Screens.Push(search)
}

func pushJellyseerrRequestScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache, result jellyseerr.SearchResult) {
	if game.Jellyseerr == nil {
		return
	}
	reqScreen := ui.NewJellyseerrRequestScreen(game.Jellyseerr, imgCache, result)
	game.Screens.Push(reqScreen)
}

func pushJellyseerrRequestsScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache) {
	if game.Jellyseerr == nil {
		return
	}
	reqsScreen := ui.NewJellyseerrRequestsScreen(game.Jellyseerr, imgCache)
	reqsScreen.OnItemSelected = func(result jellyseerr.SearchResult) {
		pushJellyseerrRequestScreen(game, cfg, imgCache, result)
	}
	reqsScreen.OnSearch = func() {
		pushJellyseerrSearchScreen(game, cfg, imgCache)
	}
	game.Screens.Push(reqsScreen)
}
