package jellyfin

import (
	"fmt"
	"net/url"
)

// GetStreamURL returns a direct-play streaming URL for an item.
func (c *Client) GetStreamURL(itemID string) string {
	params := url.Values{}
	params.Set("Static", "true")
	params.Set("api_key", c.token)
	return fmt.Sprintf("%s/Videos/%s/stream?%s",
		c.serverURL, url.PathEscape(itemID), params.Encode())
}

// GetHLSStreamURL returns an HLS streaming URL (transcoded) for an item.
func (c *Client) GetHLSStreamURL(itemID string) string {
	params := url.Values{}
	params.Set("api_key", c.token)
	params.Set("DeviceId", "jellycouch-1")
	params.Set("PlaySessionId", "jellycouch-session")
	return fmt.Sprintf("%s/Videos/%s/master.m3u8?%s",
		c.serverURL, url.PathEscape(itemID), params.Encode())
}
