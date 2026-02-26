package jellyseerr

import "github.com/depeter/jellycouch/internal/constants"

// Media status values from Jellyseerr API.
const (
	StatusUnknown            = 1
	StatusPending            = 2
	StatusPartiallyAvailable = 3
	StatusProcessing         = 4
	StatusAvailable          = 5
)

// Request status values from Jellyseerr API.
const (
	RequestPending  = 1
	RequestApproved = 2
	RequestDeclined = 3
)

// SearchResult represents a single search result from Jellyseerr (TMDB data).
type SearchResult struct {
	ID           int        `json:"id"`
	MediaType    string     `json:"mediaType"` // "movie", "tv", "person"
	Title        string     `json:"title"`     // movies
	Name         string     `json:"name"`      // tv shows
	PosterPath   string     `json:"posterPath"`
	Overview     string     `json:"overview"`
	ReleaseDate  string     `json:"releaseDate"`  // movies
	FirstAirDate string     `json:"firstAirDate"` // tv
	VoteAverage  float64    `json:"voteAverage"`
	MediaInfo    *MediaInfo `json:"mediaInfo"`
}

// DisplayTitle returns the appropriate title for the media type.
func (sr SearchResult) DisplayTitle() string {
	if sr.Title != "" {
		return sr.Title
	}
	return sr.Name
}

// Year extracts the year from the release/air date.
func (sr SearchResult) Year() string {
	d := sr.ReleaseDate
	if d == "" {
		d = sr.FirstAirDate
	}
	if len(d) >= 4 {
		return d[:4]
	}
	return ""
}

// PosterURL returns the full TMDB poster URL.
func (sr SearchResult) PosterURL() string {
	if sr.PosterPath == "" {
		return ""
	}
	return constants.TMDBPosterW300 + sr.PosterPath
}

// MediaInfo contains request/availability status for a media item.
type MediaInfo struct {
	ID       int            `json:"id"`
	Status   int            `json:"status"`
	Requests []MediaRequest `json:"requests"`
}

// MediaRequest represents a request in Jellyseerr.
type MediaRequest struct {
	ID          int          `json:"id"`
	Status      int          `json:"status"` // 1=pending, 2=approved, 3=declined
	Type        string       `json:"type"`   // "movie" or "tv"
	Media       RequestMedia `json:"media"`
	CreatedAt   string       `json:"createdAt"`
	RequestedBy RequestUser  `json:"requestedBy"`
}

// RequestMedia is the media info embedded in a request.
type RequestMedia struct {
	ID         int    `json:"id"`
	TmdbID     int    `json:"tmdbId"`
	Status     int    `json:"status"`
	MediaType  string `json:"mediaType"`
	PosterPath string `json:"posterPath"`
}

// RequestUser is the user who made a request.
type RequestUser struct {
	ID          int    `json:"id"`
	DisplayName string `json:"displayName"`
}

// RequestCount holds aggregate request counts.
type RequestCount struct {
	Total      int `json:"total"`
	Movie      int `json:"movie"`
	TV         int `json:"tv"`
	Pending    int `json:"pending"`
	Approved   int `json:"approved"`
	Declined   int `json:"declined"`
	Processing int `json:"processing"`
	Available  int `json:"available"`
}

// PagedResponse wraps a paged API response.
type PagedResponse struct {
	PageInfo PageInfo `json:"pageInfo"`
}

// PageInfo contains pagination metadata.
type PageInfo struct {
	Pages   int `json:"pages"`
	Page    int `json:"page"`
	Results int `json:"results"`
}

// SearchResponse is the response from the search endpoint.
type SearchResponse struct {
	Page         int            `json:"page"`
	TotalPages   int            `json:"totalPages"`
	TotalResults int            `json:"totalResults"`
	Results      []SearchResult `json:"results"`
}

// RequestsResponse is the response from the requests list endpoint.
type RequestsResponse struct {
	PageInfo PageInfo       `json:"pageInfo"`
	Results  []MediaRequest `json:"results"`
}

// RelatedVideo represents a video (trailer, teaser, etc.) from TMDB.
type RelatedVideo struct {
	URL  string `json:"url"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Size int    `json:"size"`
	Type string `json:"type"` // "Trailer", "Teaser", "Clip", etc.
	Site string `json:"site"` // "YouTube", etc.
}

// MovieDetail contains detailed movie info from Jellyseerr.
type MovieDetail struct {
	ID            int            `json:"id"`
	Title         string         `json:"title"`
	Overview      string         `json:"overview"`
	PosterPath    string         `json:"posterPath"`
	ReleaseDate   string         `json:"releaseDate"`
	VoteAverage   float64        `json:"voteAverage"`
	RelatedVideos []RelatedVideo `json:"relatedVideos"`
	MediaInfo     *MediaInfo     `json:"mediaInfo"`
}

// TrailerURL returns the full YouTube URL for the first trailer, or "".
func (d *MovieDetail) TrailerURL() string {
	for _, v := range d.RelatedVideos {
		if v.Type == "Trailer" && v.Site == "YouTube" && v.Key != "" {
			return "https://www.youtube.com/watch?v=" + v.Key
		}
	}
	return ""
}

// TVDetail contains detailed TV show info from Jellyseerr.
type TVDetail struct {
	ID              int            `json:"id"`
	Name            string         `json:"name"`
	Overview        string         `json:"overview"`
	PosterPath      string         `json:"posterPath"`
	FirstAirDate    string         `json:"firstAirDate"`
	VoteAverage     float64        `json:"voteAverage"`
	NumberOfSeasons int            `json:"numberOfSeasons"`
	Seasons         []Season       `json:"seasons"`
	RelatedVideos   []RelatedVideo `json:"relatedVideos"`
	MediaInfo       *MediaInfo     `json:"mediaInfo"`
}

// TrailerURL returns the full YouTube URL for the first trailer, or "".
func (d *TVDetail) TrailerURL() string {
	for _, v := range d.RelatedVideos {
		if v.Type == "Trailer" && v.Site == "YouTube" && v.Key != "" {
			return "https://www.youtube.com/watch?v=" + v.Key
		}
	}
	return ""
}

// Season represents a TV season.
type Season struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episodeCount"`
}

// CreateRequestBody is the JSON body for creating a new request.
type CreateRequestBody struct {
	MediaType         string `json:"mediaType"`
	MediaID           int    `json:"mediaId"`
	Seasons           []int  `json:"seasons,omitempty"`
	ServerID          int    `json:"serverId,omitempty"`
	ProfileID         int    `json:"profileId,omitempty"`
	RootFolder        string `json:"rootFolder,omitempty"`
	LanguageProfileID int    `json:"languageProfileId,omitempty"`
	Is4K              bool   `json:"is4k,omitempty"`
}

// RequestOptions holds optional parameters for creating a request.
type RequestOptions struct {
	ServerID          int
	ProfileID         int
	RootFolder        string
	LanguageProfileID int
	Is4K              bool
}

// ServiceProfile represents a quality profile from Radarr/Sonarr.
type ServiceProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RootFolder represents a root folder from Radarr/Sonarr.
type RootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// LanguageProfile represents a language profile from Sonarr.
type LanguageProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RadarrSettings represents a Radarr server configuration from Jellyseerr.
type RadarrSettings struct {
	ID              int              `json:"id"`
	Name            string           `json:"name"`
	IsDefault       bool             `json:"isDefault"`
	Is4K            bool             `json:"is4k"`
	ActiveProfileID int              `json:"activeProfileId"`
	ActiveDirectory string           `json:"activeDirectory"`
	Profiles        []ServiceProfile `json:"profiles"`
	RootFolders     []RootFolder     `json:"rootFolders"`
}

// SonarrSettings represents a Sonarr server configuration from Jellyseerr.
type SonarrSettings struct {
	ID                      int              `json:"id"`
	Name                    string           `json:"name"`
	IsDefault               bool             `json:"isDefault"`
	Is4K                    bool             `json:"is4k"`
	ActiveProfileID         int              `json:"activeProfileId"`
	ActiveDirectory         string           `json:"activeDirectory"`
	ActiveLanguageProfileID int              `json:"activeLanguageProfileId"`
	Profiles                []ServiceProfile `json:"profiles"`
	RootFolders             []RootFolder     `json:"rootFolders"`
	LanguageProfiles        []LanguageProfile `json:"languageProfiles"`
}

// RequestStatusLabel returns a human-readable label for a request status.
func RequestStatusLabel(status int) string {
	switch status {
	case RequestPending:
		return "Pending"
	case RequestApproved:
		return "Approved"
	case RequestDeclined:
		return "Declined"
	default:
		return "Unknown"
	}
}

// MediaStatusLabel returns a human-readable label for a media status.
func MediaStatusLabel(status int) string {
	switch status {
	case StatusPending:
		return "Pending"
	case StatusPartiallyAvailable:
		return "Partial"
	case StatusProcessing:
		return "Processing"
	case StatusAvailable:
		return "Available"
	default:
		return ""
	}
}
