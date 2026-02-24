package jellyfin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	jellyfin "github.com/sj14/jellyfin-go/api"
)

const (
	clientName    = "JellyCouch"
	clientVersion = "0.1.0"
	deviceName    = "JellyCouch Desktop"
)

// Client wraps the generated Jellyfin API client with convenience methods.
type Client struct {
	api       *jellyfin.APIClient
	ctx       context.Context
	token     string
	userID    string
	serverURL string
}

func normalizeURL(serverURL string) string {
	serverURL = strings.TrimSpace(serverURL)
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "https://" + serverURL
	}
	return strings.TrimRight(serverURL, "/")
}

func NewClient(serverURL string) *Client {
	serverURL = normalizeURL(serverURL)
	cfg := jellyfin.NewConfiguration()
	cfg.Servers = jellyfin.ServerConfigurations{
		{URL: serverURL},
	}
	cfg.AddDefaultHeader("X-Emby-Authorization",
		fmt.Sprintf(`MediaBrowser Client="%s", Device="%s", DeviceId="jellycouch-1", Version="%s"`,
			clientName, deviceName, clientVersion))

	return &Client{
		api:       jellyfin.NewAPIClient(cfg),
		ctx:       context.Background(),
		serverURL: serverURL,
	}
}

func (c *Client) Authenticate(username, password string) error {
	body := *jellyfin.NewAuthenticateUserByName()
	body.SetUsername(username)
	body.SetPw(password)

	result, resp, err := c.api.UserAPI.AuthenticateUserByName(c.ctx).AuthenticateUserByName(body).Execute()
	if err != nil {
		return fmt.Errorf("auth failed: %w (status: %s)", err, respStatus(resp))
	}
	c.token = result.GetAccessToken()
	user := result.GetUser()
	if user.Id != nil {
		c.userID = *user.Id
	}

	// Update config with token for subsequent requests
	c.api.GetConfig().AddDefaultHeader("X-Emby-Token", c.token)
	return nil
}

func (c *Client) SetToken(token, userID string) {
	c.token = token
	c.userID = userID
	c.api.GetConfig().AddDefaultHeader("X-Emby-Token", c.token)
}

func (c *Client) Token() string            { return c.token }
func (c *Client) UserID() string           { return c.userID }
func (c *Client) ServerURL() string        { return c.serverURL }
func (c *Client) API() *jellyfin.APIClient { return c.api }
func (c *Client) Context() context.Context { return c.ctx }

func respStatus(resp *http.Response) string {
	if resp == nil {
		return "no response"
	}
	return resp.Status
}
