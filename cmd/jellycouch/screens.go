package main

import (
	"log"

	"github.com/depeter/jellycouch/internal/app"
	"github.com/depeter/jellycouch/internal/cache"
	"github.com/depeter/jellycouch/internal/config"
	"github.com/depeter/jellycouch/internal/jellyfin"
	"github.com/depeter/jellycouch/internal/jellyseerr"
	"github.com/depeter/jellycouch/internal/ui"
)

// screenFactory captures the shared dependencies for creating and wiring screens.
type screenFactory struct {
	game     *app.Game
	cfg      *config.Config
	imgCache *cache.ImageCache
}

func (sf *screenFactory) pushLogin(navbar *ui.NavBar) {
	loginScreen := ui.NewLoginScreen(sf.cfg.Server.URL, func(screen *ui.LoginScreen, server, user, pass string) {
		screen.Busy = true
		screen.Error = ""
		go func() {
			c := jellyfin.NewClient(server)
			if err := c.Authenticate(user, pass); err != nil {
				screen.Error = "Login failed: " + err.Error()
				screen.Busy = false
				return
			}
			sf.cfg.Server.URL = server
			sf.cfg.Server.Username = user
			sf.cfg.Server.Token = c.Token()
			sf.cfg.Server.UserID = c.UserID()
			sf.cfg.Save()

			sf.game.Client = c
			screen.Busy = false
			sf.pushHome()
			sf.loadNavBarViews()
		}()
	})
	sf.game.Screens.Replace(loginScreen)
}

func (sf *screenFactory) pushHome() {
	home := ui.NewHomeScreen(sf.game.Client, sf.imgCache)
	home.OnItemSelected = func(item jellyfin.MediaItem) {
		sf.pushDetail(item)
	}
	home.OnLibraryBrowse = func(parentID, title string) {
		sf.pushLibrary(parentID, title, nil)
	}
	home.OnAuthError = func() {
		sf.pushLogin(sf.game.Screens.NavBar)
	}
	sf.game.Screens.Replace(home)
}

func (sf *screenFactory) pushDetail(item jellyfin.MediaItem) {
	detail := ui.NewDetailScreen(sf.game.Client, sf.imgCache, item)
	detail.OnPlay = func(item jellyfin.MediaItem, resumeTicks int64) {
		sf.game.StartPlayback(item.ID, resumeTicks, &item)
	}
	detail.OnLibrary = func(parentID, title string) {
		sf.pushLibrary(parentID, title, nil)
	}
	sf.game.Screens.Push(detail)
}

func (sf *screenFactory) pushLibrary(parentID, title string, itemTypes []string) {
	lib := ui.NewLibraryScreen(sf.game.Client, sf.imgCache, parentID, title, itemTypes)
	lib.OnItemSelected = func(item jellyfin.MediaItem) {
		sf.pushDetail(item)
	}
	sf.game.Screens.Push(lib)
}

func (sf *screenFactory) pushSearch(query string) {
	search := ui.NewSearchScreen(sf.game.Client, sf.imgCache)
	search.OnItemSelected = func(item jellyfin.MediaItem) {
		sf.pushDetail(item)
	}
	if query != "" {
		search.SetInitialQuery(query)
	}
	sf.game.Screens.Push(search)
}

func (sf *screenFactory) pushSettings() {
	settings := ui.NewSettingsScreen(sf.cfg, func() {
		sf.cfg.Save()
		if sf.cfg.Jellyseerr.URL != "" && sf.cfg.Jellyseerr.APIKey != "" {
			sf.game.Jellyseerr = jellyseerr.NewClient(sf.cfg.Jellyseerr.URL, sf.cfg.Jellyseerr.APIKey)
		} else {
			sf.game.Jellyseerr = nil
		}
		sf.loadNavBarViews()
	})
	sf.game.Screens.Push(settings)
}

func (sf *screenFactory) pushJellyseerrDiscover() {
	if sf.game.Jellyseerr == nil {
		return
	}
	discover := ui.NewJellyseerrDiscoverScreen(sf.game.Jellyseerr, sf.imgCache)
	discover.OnItemSelected = func(result jellyseerr.SearchResult) {
		sf.pushJellyseerrRequest(result)
	}
	discover.OnRequests = func() {
		sf.pushJellyseerrRequests()
	}
	discover.OnSearch = func() {
		sf.pushJellyseerrSearch()
	}
	sf.game.Screens.Push(discover)
}

func (sf *screenFactory) pushJellyseerrSearch() {
	if sf.game.Jellyseerr == nil {
		return
	}
	search := ui.NewJellyseerrSearchScreen(sf.game.Jellyseerr, sf.imgCache)
	search.OnResultSelected = func(result jellyseerr.SearchResult) {
		sf.pushJellyseerrRequest(result)
	}
	sf.game.Screens.Push(search)
}

func (sf *screenFactory) pushJellyseerrRequest(result jellyseerr.SearchResult) {
	if sf.game.Jellyseerr == nil {
		return
	}
	reqScreen := ui.NewJellyseerrRequestScreen(sf.game.Jellyseerr, sf.imgCache, result)
	reqScreen.OnPlayTrailer = func(url string) {
		sf.game.PlayURL(url)
	}
	sf.game.Screens.Push(reqScreen)
}

func (sf *screenFactory) pushJellyseerrRequests() {
	if sf.game.Jellyseerr == nil {
		return
	}
	reqsScreen := ui.NewJellyseerrRequestsScreen(sf.game.Jellyseerr, sf.imgCache)
	reqsScreen.OnItemSelected = func(result jellyseerr.SearchResult) {
		sf.pushJellyseerrRequest(result)
	}
	reqsScreen.OnSearch = func() {
		sf.pushJellyseerrSearch()
	}
	sf.game.Screens.Push(reqsScreen)
}

func (sf *screenFactory) loadNavBarViews() {
	if sf.game.Client == nil {
		return
	}
	go func() {
		views, err := sf.game.Client.GetViews()
		if err != nil {
			log.Printf("NavBar: failed to load views: %v", err)
			return
		}
		var libViews []struct{ ID, Name string }
		for _, v := range views {
			libViews = append(libViews, struct{ ID, Name string }{v.ID, v.Name})
		}
		sf.game.Screens.NavBar.LibraryViews = libViews
	}()
}
