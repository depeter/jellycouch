package jellyseerr

import "fmt"

// CreateRequest creates a new media request in Jellyseerr.
func (c *Client) CreateRequest(mediaID int, mediaType string, seasons []int, opts *RequestOptions) (*MediaRequest, error) {
	body := CreateRequestBody{
		MediaType: mediaType,
		MediaID:   mediaID,
		Seasons:   seasons,
	}
	if opts != nil {
		body.ServerID = opts.ServerID
		body.ProfileID = opts.ProfileID
		body.RootFolder = opts.RootFolder
		body.LanguageProfileID = opts.LanguageProfileID
		body.Is4K = opts.Is4K
	}
	var result MediaRequest
	if err := c.post("/api/v1/request", body, &result); err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return &result, nil
}

// GetRequests fetches a list of requests with optional status filter.
// filter can be: "all", "approved", "pending", "declined", "processing", "available" or empty for all.
func (c *Client) GetRequests(filter string, take, skip int) ([]MediaRequest, int, error) {
	path := fmt.Sprintf("/api/v1/request?take=%d&skip=%d&sort=added", take, skip)
	if filter != "" && filter != "all" {
		path += "&filter=" + filter
	}
	var resp RequestsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, 0, fmt.Errorf("get requests: %w", err)
	}
	return resp.Results, resp.PageInfo.Results, nil
}

// GetRequestCount returns aggregate counts of requests by status.
func (c *Client) GetRequestCount() (*RequestCount, error) {
	var count RequestCount
	if err := c.get("/api/v1/request/count", &count); err != nil {
		return nil, fmt.Errorf("get request count: %w", err)
	}
	return &count, nil
}
