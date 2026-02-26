package jellyseerr

import "fmt"

// GetMovie fetches detailed movie info by TMDB ID, including request status.
func (c *Client) GetMovie(tmdbID int) (*MovieDetail, error) {
	var detail MovieDetail
	if err := c.get(fmt.Sprintf("%s/%d", pathMovie, tmdbID), &detail); err != nil {
		return nil, fmt.Errorf("get movie: %w", err)
	}
	return &detail, nil
}

// GetTV fetches detailed TV show info by TMDB ID, including request status.
func (c *Client) GetTV(tmdbID int) (*TVDetail, error) {
	var detail TVDetail
	if err := c.get(fmt.Sprintf("%s/%d", pathTV, tmdbID), &detail); err != nil {
		return nil, fmt.Errorf("get tv: %w", err)
	}
	return &detail, nil
}
