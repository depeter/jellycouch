package opensubtitles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const baseURL = "https://api.opensubtitles.com/api/v1"

// Client handles communication with the OpenSubtitles REST API.
type Client struct {
	apiKey, username, password string
	httpClient                 *http.Client

	mu        sync.Mutex
	token     string
	tokenTime time.Time
}

// NewClient creates a new OpenSubtitles client.
func NewClient(apiKey, username, password string) *Client {
	return &Client{
		apiKey:   apiKey,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Login authenticates with the API and caches the JWT token.
func (c *Client) Login() error {
	var resp loginResponse
	if err := c.post("/login", loginRequest{Username: c.username, Password: c.password}, &resp); err != nil {
		return fmt.Errorf("opensubtitles login: %w", err)
	}
	if resp.Token == "" {
		return fmt.Errorf("opensubtitles login: empty token")
	}
	c.mu.Lock()
	c.token = resp.Token
	c.tokenTime = time.Now()
	c.mu.Unlock()
	return nil
}

// ensureToken logs in if we have no token or the token is ≥23h old.
func (c *Client) ensureToken() error {
	c.mu.Lock()
	needLogin := c.token == "" || time.Since(c.tokenTime) >= 23*time.Hour
	c.mu.Unlock()
	if needLogin {
		return c.Login()
	}
	return nil
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "JellyCouch/1.0")

	c.mu.Lock()
	tok := c.token
	c.mu.Unlock()
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req, nil
}

func (c *Client) get(path string, dst any) error {
	if err := c.ensureToken(); err != nil {
		return err
	}
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.do(req, dst)
}

func (c *Client) post(path string, body, dst any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := c.newRequest(http.MethodPost, path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	return c.do(req, dst)
}

func (c *Client) do(req *http.Request, dst any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("opensubtitles %s %s: status %d: %s",
			req.Method, req.URL.Path, resp.StatusCode, body)
	}

	if dst != nil {
		return json.NewDecoder(resp.Body).Decode(dst)
	}
	return nil
}
