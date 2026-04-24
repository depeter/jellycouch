package subs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// OpenSubtitles implements Provider against the opensubtitles.com REST API v1
// (https://opensubtitles.stoplight.io/docs/opensubtitles-api). Requires an API
// key (free tier is fine for personal use). Username/password are optional and
// only needed to raise the daily download quota.
type OpenSubtitles struct {
	APIKey    string
	Username  string
	Password  string
	UserAgent string

	http    *http.Client
	token   string
	tokenAt time.Time
}

// NewOpenSubtitles builds a provider. The API key is required; username and
// password may be empty (anonymous access allows a handful of downloads/day).
func NewOpenSubtitles(apiKey, username, password string) *OpenSubtitles {
	return &OpenSubtitles{
		APIKey:    apiKey,
		Username:  username,
		Password:  password,
		UserAgent: "JellyCouch v0.1",
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenSubtitles) Name() string { return "OpenSubtitles" }

const opensubtitlesBase = "https://api.opensubtitles.com/api/v1"

// osSearchResponse mirrors the subset of fields we use from /subtitles.
// Field names match the API exactly so encoding/json populates them.
type osSearchResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Language        string  `json:"language"`
			DownloadCount   int     `json:"download_count"`
			Ratings         float64 `json:"ratings"`
			Release         string  `json:"release"`
			HearingImpaired bool    `json:"hearing_impaired"`
			FeatureDetails  struct {
				Title        string `json:"title"`
				MovieName    string `json:"movie_name"`
				SeasonNumber int    `json:"season_number"`
				EpisodeNumber int   `json:"episode_number"`
			} `json:"feature_details"`
			Files []struct {
				FileID   int    `json:"file_id"`
				FileName string `json:"file_name"`
			} `json:"files"`
		} `json:"attributes"`
	} `json:"data"`
}

// osDownloadResponse is the response from POST /download.
type osDownloadResponse struct {
	Link       string `json:"link"`
	FileName   string `json:"file_name"`
	Requests   int    `json:"requests"`
	Remaining  int    `json:"remaining"`
	ResetTime  string `json:"reset_time"`
	Message    string `json:"message"`
}

// osLoginResponse is the response from POST /login.
type osLoginResponse struct {
	Token  string `json:"token"`
	Status int    `json:"status"`
}

// osProviderData is what OpenSubtitles stores on a Result so Download can
// fetch the actual .srt afterwards.
type osProviderData struct {
	FileID   int
	FileName string
}

// newReq builds an authenticated request. OpenSubtitles requires Api-Key and
// User-Agent on every call; once logged in, Bearer token unlocks the
// higher download quota.
func (p *OpenSubtitles) newReq(method, u string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", p.APIKey)
	req.Header.Set("User-Agent", p.UserAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	return req, nil
}

// ensureLogin logs in if credentials are set and we have no valid token.
// Silently skips if no credentials — anonymous requests still work, with
// lower quota.
func (p *OpenSubtitles) ensureLogin() {
	if p.Username == "" || p.Password == "" {
		return
	}
	if p.token != "" && time.Since(p.tokenAt) < 23*time.Hour {
		return
	}
	body, _ := json.Marshal(map[string]string{
		"username": p.Username,
		"password": p.Password,
	})
	req, err := p.newReq("POST", opensubtitlesBase+"/login", strings.NewReader(string(body)))
	if err != nil {
		return
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var lr osLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return
	}
	p.token = lr.Token
	p.tokenAt = time.Now()
}

// Search queries the /subtitles endpoint. OpenSubtitles accepts either an
// imdb_id or a query + year combination; we pass whichever we have.
func (p *OpenSubtitles) Search(q Query) ([]Result, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("opensubtitles: api key not set")
	}
	p.ensureLogin()

	vals := url.Values{}
	if q.IMDBID != "" {
		vals.Set("imdb_id", strings.TrimPrefix(q.IMDBID, "tt"))
	}
	if q.Title != "" {
		vals.Set("query", q.Title)
	}
	if q.Year > 0 {
		vals.Set("year", strconv.Itoa(q.Year))
	}
	if q.IsEpisode() {
		vals.Set("season_number", strconv.Itoa(q.Season))
		vals.Set("episode_number", strconv.Itoa(q.Episode))
	}
	if len(q.Languages) > 0 {
		// API expects 2-letter codes; convert from 3-letter where possible.
		var codes []string
		for _, l := range q.Languages {
			codes = append(codes, iso639_2to1(l))
		}
		vals.Set("languages", strings.Join(codes, ","))
	}

	req, err := p.newReq("GET", opensubtitlesBase+"/subtitles?"+vals.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensubtitles search: %s: %s", resp.Status, truncate(string(bodyBytes), 200))
	}
	var sr osSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("opensubtitles decode: %w", err)
	}
	results := make([]Result, 0, len(sr.Data))
	for _, d := range sr.Data {
		if len(d.Attributes.Files) == 0 {
			continue
		}
		f := d.Attributes.Files[0]
		title := d.Attributes.FeatureDetails.Title
		if title == "" {
			title = d.Attributes.FeatureDetails.MovieName
		}
		results = append(results, Result{
			Provider:        p.Name(),
			Language:        iso639_1to2(d.Attributes.Language),
			ReleaseName:     d.Attributes.Release,
			Title:           title,
			Downloads:       d.Attributes.DownloadCount,
			Rating:          d.Attributes.Ratings,
			HearingImpaired: d.Attributes.HearingImpaired,
			providerData: osProviderData{
				FileID:   f.FileID,
				FileName: f.FileName,
			},
		})
	}
	return results, nil
}

// Download POSTs to /download with the file_id to get a one-time URL, then
// fetches the .srt body and writes it to destDir.
func (p *OpenSubtitles) Download(r Result, destDir string) (string, error) {
	data, ok := r.providerData.(osProviderData)
	if !ok {
		return "", fmt.Errorf("opensubtitles: result missing file_id")
	}
	p.ensureLogin()

	reqBody, _ := json.Marshal(map[string]int{"file_id": data.FileID})
	req, err := p.newReq("POST", opensubtitlesBase+"/download", strings.NewReader(string(reqBody)))
	if err != nil {
		return "", err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("opensubtitles download: %s: %s", resp.Status, truncate(string(body), 200))
	}
	var dr osDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return "", fmt.Errorf("opensubtitles download decode: %w", err)
	}
	if dr.Link == "" {
		return "", fmt.Errorf("opensubtitles download: no link (%s)", dr.Message)
	}

	// Fetch the raw file — this URL is a pre-signed short-lived download link.
	fileResp, err := p.http.Get(dr.Link)
	if err != nil {
		return "", err
	}
	defer fileResp.Body.Close()
	if fileResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("opensubtitles fetch: %s", fileResp.Status)
	}
	body, err := io.ReadAll(fileResp.Body)
	if err != nil {
		return "", err
	}

	ext := ".srt"
	if idx := strings.LastIndex(dr.FileName, "."); idx >= 0 {
		ext = dr.FileName[idx:]
	}
	path := cachePath(destDir, p.Name(), r.Language, strconv.Itoa(data.FileID), ext)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
