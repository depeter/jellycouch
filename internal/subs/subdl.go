package subs

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// Subdl implements Provider against the Subdl API (https://subdl.com).
// Subdl serves subtitles as zip archives containing one or more .srt files.
// We download the archive in memory and extract the first matching .srt.
type Subdl struct {
	APIKey string
	http   *http.Client
}

func NewSubdl(apiKey string) *Subdl {
	return &Subdl{
		APIKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Subdl) Name() string { return "Subdl" }

const (
	subdlAPIBase = "https://api.subdl.com/api/v1/subtitles"
	subdlDLBase  = "https://dl.subdl.com"
)

type subdlSearchResponse struct {
	Status    bool           `json:"status"`
	Error     string         `json:"error"`
	Subtitles []subdlSubtitle `json:"subtitles"`
}

type subdlSubtitle struct {
	ReleaseName string `json:"release_name"`
	Name        string `json:"name"`
	Lang        string `json:"lang"`      // "English"
	Language    string `json:"language"`  // "EN"
	Author      string `json:"author"`
	URL         string `json:"url"`       // relative zip path
	Season      int    `json:"season"`
	Episode     int    `json:"episode"`
	HI          bool   `json:"hi"`        // hearing impaired
	FullSeason  bool   `json:"full_season"`
}

// subdlProviderData holds the zip URL and language for later Download.
type subdlProviderData struct {
	URL string
}

func (p *Subdl) Search(q Query) ([]Result, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("subdl: api key not set")
	}

	vals := url.Values{}
	vals.Set("api_key", p.APIKey)
	if q.IMDBID != "" {
		vals.Set("imdb_id", "tt"+strings.TrimPrefix(q.IMDBID, "tt"))
	}
	if q.Title != "" {
		vals.Set("film_name", q.Title)
	}
	if q.Year > 0 {
		vals.Set("year", strconv.Itoa(q.Year))
	}
	if q.IsEpisode() {
		if q.Season > 0 {
			vals.Set("season_number", strconv.Itoa(q.Season))
		}
		if q.Episode > 0 {
			vals.Set("episode_number", strconv.Itoa(q.Episode))
		}
		vals.Set("type", "tv")
	}
	if len(q.Languages) > 0 {
		var codes []string
		for _, l := range q.Languages {
			codes = append(codes, strings.ToUpper(iso639_2to1(l)))
		}
		vals.Set("languages", strings.Join(codes, ","))
	}
	// Subdl caps at 30 per page by default; that's plenty for our UI.

	req, err := http.NewRequest("GET", subdlAPIBase+"?"+vals.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("subdl search: %s: %s", resp.Status, truncate(string(body), 200))
	}
	var sr subdlSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("subdl decode: %w", err)
	}
	if !sr.Status {
		if sr.Error != "" {
			return nil, fmt.Errorf("subdl: %s", sr.Error)
		}
		return nil, nil
	}
	results := make([]Result, 0, len(sr.Subtitles))
	for _, s := range sr.Subtitles {
		results = append(results, Result{
			Provider:        p.Name(),
			Language:        iso639_1to2(strings.ToLower(s.Language)),
			ReleaseName:     s.ReleaseName,
			Title:           s.Name,
			HearingImpaired: s.HI,
			providerData:    subdlProviderData{URL: s.URL},
		})
	}
	return results, nil
}

// Download fetches the zip archive, extracts the first .srt within, and
// writes it to destDir.
func (p *Subdl) Download(r Result, destDir string) (string, error) {
	data, ok := r.providerData.(subdlProviderData)
	if !ok || data.URL == "" {
		return "", fmt.Errorf("subdl: result missing url")
	}

	fullURL := data.URL
	if strings.HasPrefix(fullURL, "/") {
		fullURL = subdlDLBase + fullURL
	}

	resp, err := p.http.Get(fullURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("subdl fetch: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("subdl unzip: %w", err)
	}

	// Pick the first entry ending in .srt (or .ass/.vtt as a fallback).
	var entry *zip.File
	for _, f := range zr.File {
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".srt") {
			entry = f
			break
		}
	}
	if entry == nil {
		for _, f := range zr.File {
			lower := strings.ToLower(f.Name)
			if strings.HasSuffix(lower, ".ass") || strings.HasSuffix(lower, ".vtt") || strings.HasSuffix(lower, ".ssa") {
				entry = f
				break
			}
		}
	}
	if entry == nil {
		return "", fmt.Errorf("subdl: no subtitle files in archive")
	}

	rc, err := entry.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	subData, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	ext := ".srt"
	if idx := strings.LastIndex(entry.Name, "."); idx >= 0 {
		ext = strings.ToLower(entry.Name[idx:])
	}
	// Use the zip URL's basename (minus .zip) as the identifier for the cache
	// path, so re-downloading the same result overwrites the cached file.
	base := strings.TrimSuffix(path.Base(data.URL), ".zip")
	out := cachePath(destDir, p.Name(), r.Language, base, ext)
	if err := os.WriteFile(out, subData, 0o644); err != nil {
		return "", err
	}
	return out, nil
}
