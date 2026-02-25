package jellyseerr

import (
	"fmt"
	"net/url"
)

// Search queries Jellyseerr for media matching the given query string.
func (c *Client) Search(query string, page int) (*SearchResponse, error) {
	path := fmt.Sprintf("/api/v1/search?query=%s&page=%d&language=en", url.QueryEscape(query), page)
	var resp SearchResponse
	if err := c.get(path, &resp); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return &resp, nil
}

// GetTrending fetches trending media from Jellyseerr's discover endpoint.
func (c *Client) GetTrending(page int) (*SearchResponse, error) {
	path := fmt.Sprintf("/api/v1/discover/trending?page=%d&language=en", page)
	var resp SearchResponse
	if err := c.get(path, &resp); err != nil {
		return nil, fmt.Errorf("get trending: %w", err)
	}
	return &resp, nil
}

// GetDiscoverMovies fetches popular movies from Jellyseerr's discover endpoint.
func (c *Client) GetDiscoverMovies(page int) (*SearchResponse, error) {
	path := fmt.Sprintf("/api/v1/discover/movies?page=%d&language=en", page)
	var resp SearchResponse
	if err := c.get(path, &resp); err != nil {
		return nil, fmt.Errorf("get discover movies: %w", err)
	}
	return &resp, nil
}

// GetDiscoverTV fetches popular TV shows from Jellyseerr's discover endpoint.
func (c *Client) GetDiscoverTV(page int) (*SearchResponse, error) {
	path := fmt.Sprintf("/api/v1/discover/tv?page=%d&language=en", page)
	var resp SearchResponse
	if err := c.get(path, &resp); err != nil {
		return nil, fmt.Errorf("get discover tv: %w", err)
	}
	return &resp, nil
}
