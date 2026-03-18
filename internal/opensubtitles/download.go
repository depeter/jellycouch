package opensubtitles

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// GetDownloadLink requests a one-time download link for the given file ID.
func (c *Client) GetDownloadLink(fileID int) (*DownloadResponse, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}
	var resp DownloadResponse
	if err := c.post("/download", downloadRequest{FileID: fileID}, &resp); err != nil {
		return nil, fmt.Errorf("get download link: %w", err)
	}
	if resp.Link == "" {
		return nil, fmt.Errorf("get download link: empty link in response")
	}
	return &resp, nil
}

// Download downloads a subtitle file to destPath.
// It calls GetDownloadLink, then fetches the file and writes it to destPath,
// creating parent directories as needed.
func (c *Client) Download(fileID int, destPath string) error {
	dlResp, err := c.GetDownloadLink(fileID)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create subtitle dir: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, dlResp.Link, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", "JellyCouch/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download subtitle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download subtitle: status %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create subtitle file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write subtitle file: %w", err)
	}
	return nil
}
