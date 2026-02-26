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

	// Create and wire the global navbar
	navbar := ui.NewNavBar()
	navbar.OnNavigate = func(action, id, title string) {
		switch action {
		case "home":
			game.Screens.ClearStack()
			pushHomeScreen(game, cfg, imgCache)
		case "library":
			game.Screens.ClearStack()
			pushHomeScreen(game, cfg, imgCache)
			pushLibraryScreen(game, cfg, imgCache, id, title, nil)
		case "discovery":
			game.Screens.ClearStack()
			pushHomeScreen(game, cfg, imgCache)
			pushJellyseerrDiscoverScreen(game, cfg, imgCache)
		case "settings":
			game.Screens.ClearStack()
			pushHomeScreen(game, cfg, imgCache)
			pushSettingsScreen(game, cfg)
		}
	}
	navbar.OnSearch = func(query string) {
		game.Screens.ClearStack()
		pushHomeScreen(game, cfg, imgCache)
		pushSearchScreen(game, cfg, imgCache, query)
	}
	navbar.JellyseerrEnabled = func() bool {
		return game.Jellyseerr != nil
	}
	game.Screens.NavBar = navbar

	// Determine initial screen
	if client == nil || cfg.Server.Token == "" {
		pushLoginScreen(game, cfg, imgCache, navbar)
	} else {
		// Validate token before showing home screen
		if _, err := client.GetViews(); err != nil {
			log.Printf("Token invalid, showing login: %v", err)
			pushLoginScreen(game, cfg, imgCache, navbar)
		} else {
			pushHomeScreen(game, cfg, imgCache)
			loadNavBarViews(game.Client, navbar)
		}
	}

	// Configure window
	ebiten.SetWindowSize(cfg.UI.Width, cfg.UI.Height)
	ebiten.SetWindowTitle("JellyCouch")
	ebiten.SetWindowIcon(icon.Generate())
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if cfg.UI.Fullscreen {
		ebiten.SetFullscreen(true)
	}

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

// loadNavBarViews fetches library views and populates the navbar buttons.
func loadNavBarViews(client *jellyfin.Client, navbar *ui.NavBar) {
	if client == nil {
		return
	}
	go func() {
		views, err := client.GetViews()
		if err != nil {
			log.Printf("NavBar: failed to load views: %v", err)
			return
		}
		var libViews []struct{ ID, Name string }
		for _, v := range views {
			libViews = append(libViews, struct{ ID, Name string }{v.ID, v.Name})
		}
		navbar.LibraryViews = libViews
	}()
}

func pushLoginScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache, navbar *ui.NavBar) {
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
			loadNavBarViews(c, navbar)
		}()
	})
	game.Screens.Replace(loginScreen)
}

func pushHomeScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache) {
	home := ui.NewHomeScreen(game.Client, imgCache)
	home.OnItemSelected = func(item jellyfin.MediaItem) {
		pushDetailScreen(game, cfg, imgCache, item)
	}
	home.OnLibraryBrowse = func(parentID, title string) {
		pushLibraryScreen(game, cfg, imgCache, parentID, title, nil)
	}
	home.OnAuthError = func() {
		pushLoginScreen(game, cfg, imgCache, game.Screens.NavBar)
	}
	game.Screens.Replace(home)
}

func pushDetailScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache, item jellyfin.MediaItem) {
	detail := ui.NewDetailScreen(game.Client, imgCache, item)
	detail.OnPlay = func(item jellyfin.MediaItem, resumeTicks int64) {
		game.StartPlayback(item.ID, resumeTicks, &item)
	}
	detail.OnLibrary = func(parentID, title string) {
		pushLibraryScreen(game, cfg, imgCache, parentID, title, nil)
	}
	game.Screens.Push(detail)
}

func pushLibraryScreen(game *app.Game, cfg *config.Config, imgCache *cache.ImageCache, parentID, title string, itemTypes []string) {
	lib := ui.NewLibraryScreen(game.Client, imgCache, parentID, title, itemTypes)
	lib.OnItemSelected = func(item jellyfin.MediaItem) {
		pushDetailScreen(game, cfg, imgCache, item)
	}
	game.Screens.Push(lib)
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
		// Reload navbar views in case server changed
		loadNavBarViews(game.Client, game.Screens.NavBar)
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
	reqScreen.OnPlayTrailer = func(url string) {
		game.PlayURL(url)
	}
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
