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
