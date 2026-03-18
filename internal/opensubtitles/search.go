package opensubtitles

import (
	"fmt"
	"net/url"
	"strings"
)

// SearchParams holds parameters for a subtitle search.
type SearchParams struct {
	IMDBID        string // "tt1234567" — strip "tt" prefix for API
	Query         string // fallback title search
	Year          int
	Languages     string // comma-separated, e.g. "en"
	Type          string // "movie" or "episode"
	SeasonNumber  int
	EpisodeNumber int
}

// Search searches for subtitles matching the given parameters.
func (c *Client) Search(p SearchParams) (*SearchResponse, error) {
	q := url.Values{}

	if p.IMDBID != "" {
		// Strip "tt" prefix — the API wants only the numeric part
		imdbNum := strings.TrimPrefix(p.IMDBID, "tt")
		q.Set("imdb_id", imdbNum)
	} else if p.Query != "" {
		q.Set("query", p.Query)
		if p.Year > 0 {
			q.Set("year", fmt.Sprintf("%d", p.Year))
		}
	}

	if p.Languages != "" {
		q.Set("languages", p.Languages)
	}
	if p.Type != "" {
		q.Set("type", p.Type)
	}
	if p.SeasonNumber > 0 {
		q.Set("season_number", fmt.Sprintf("%d", p.SeasonNumber))
	}
	if p.EpisodeNumber > 0 {
		q.Set("episode_number", fmt.Sprintf("%d", p.EpisodeNumber))
	}

	path := "/subtitles"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var resp SearchResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
