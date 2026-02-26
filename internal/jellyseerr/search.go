package jellyseerr

import (
	"fmt"
	"net/url"
)

func discoverParams(page int) string {
	v := url.Values{}
	v.Set("page", fmt.Sprintf("%d", page))
	v.Set("language", "en")
	return "?" + v.Encode()
}

// Search queries Jellyseerr for media matching the given query string.
func (c *Client) Search(query string, page int) (*SearchResponse, error) {
	v := url.Values{}
	v.Set("query", query)
	v.Set("page", fmt.Sprintf("%d", page))
	v.Set("language", "en")
	var resp SearchResponse
	if err := c.get(pathSearch+"?"+v.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return &resp, nil
}

// GetTrending fetches trending media from Jellyseerr's discover endpoint.
func (c *Client) GetTrending(page int) (*SearchResponse, error) {
	var resp SearchResponse
	if err := c.get(pathTrending+discoverParams(page), &resp); err != nil {
		return nil, fmt.Errorf("get trending: %w", err)
	}
	return &resp, nil
}

// GetDiscoverMovies fetches popular movies from Jellyseerr's discover endpoint.
func (c *Client) GetDiscoverMovies(page int) (*SearchResponse, error) {
	var resp SearchResponse
	if err := c.get(pathDiscoverMovies+discoverParams(page), &resp); err != nil {
		return nil, fmt.Errorf("get discover movies: %w", err)
	}
	return &resp, nil
}

// GetDiscoverTV fetches popular TV shows from Jellyseerr's discover endpoint.
func (c *Client) GetDiscoverTV(page int) (*SearchResponse, error) {
	var resp SearchResponse
	if err := c.get(pathDiscoverTV+discoverParams(page), &resp); err != nil {
		return nil, fmt.Errorf("get discover tv: %w", err)
	}
	return &resp, nil
}
