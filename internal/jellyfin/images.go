package jellyfin

import (
	"fmt"
	"net/url"
)

// ImageType represents different image types.
type ImageType string

const (
	ImagePrimary  ImageType = "Primary"
	ImageBackdrop ImageType = "Backdrop"
	ImageThumb    ImageType = "Thumb"
	ImageBanner   ImageType = "Banner"
	ImageLogo     ImageType = "Logo"
)

// GetImageURL constructs a URL for an item's image.
func (c *Client) GetImageURL(itemID string, imgType ImageType, maxWidth, maxHeight int) string {
	u := fmt.Sprintf("%s/Items/%s/Images/%s", c.serverURL, url.PathEscape(itemID), string(imgType))
	params := url.Values{}
	if maxWidth > 0 {
		params.Set("maxWidth", fmt.Sprintf("%d", maxWidth))
	}
	if maxHeight > 0 {
		params.Set("maxHeight", fmt.Sprintf("%d", maxHeight))
	}
	params.Set("quality", "90")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u
}

// GetPosterURL returns the primary image URL sized for poster display.
func (c *Client) GetPosterURL(itemID string) string {
	return c.GetImageURL(itemID, ImagePrimary, 300, 450)
}

// GetBackdropURL returns the backdrop image URL.
func (c *Client) GetBackdropURL(itemID string) string {
	return c.GetImageURL(itemID, ImageBackdrop, 1920, 1080)
}
