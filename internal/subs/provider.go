// Package subs finds and downloads external subtitle files from third-party
// providers (OpenSubtitles, Subdl, ...) so that the player can load them via
// mpv's sub-add command.
package subs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Query describes what to search for. Providers use whichever fields they
// support; unknown fields are ignored.
type Query struct {
	// Title of the movie or series. For episodes this is the *show* name
	// (e.g. "Breaking Bad"), not the episode title.
	Title string
	// Year of release (0 if unknown).
	Year int
	// Episode metadata — zero for movies.
	Season  int
	Episode int
	// IMDBID without the "tt" prefix (e.g. "0903747"). Optional but preferred
	// when available — providers tend to return much better matches by ID.
	IMDBID string
	// Languages to include, in ISO 639-2 3-letter codes (e.g. "eng", "dut").
	// Empty means "any".
	Languages []string
}

// IsEpisode reports whether the query targets a TV episode (as opposed to a movie).
func (q Query) IsEpisode() bool {
	return q.Season > 0 || q.Episode > 0
}

// Result is a single searched subtitle returned by a provider.
type Result struct {
	// Provider is the display name of the source (e.g. "OpenSubtitles").
	Provider string
	// Language code in ISO 639-2 form when possible.
	Language string
	// ReleaseName is the uploader's description of the source release,
	// e.g. "Breaking.Bad.S01E01.720p.BluRay.x264-REWARD".
	ReleaseName string
	// Title is the movie or episode title as the provider returns it.
	Title string
	// Downloads/Rating are optional sort signals; 0 if unknown.
	Downloads int
	Rating    float64
	// HearingImpaired is true for SDH/closed-caption variants.
	HearingImpaired bool
	// providerData carries anything the provider needs to hand back to its
	// Download method (file_id, direct URL, ...). Opaque to other providers.
	providerData any
}

// Provider is the minimum interface a subtitle source must implement.
type Provider interface {
	// Name is a short, stable identifier used in config and UI.
	Name() string
	// Search returns results matching q. Callers treat an empty slice and
	// an error equivalently — callers should fall through to other providers.
	Search(q Query) ([]Result, error)
	// Download writes the subtitle file for r under destDir and returns the
	// absolute path. The caller hands the path straight to mpv's sub-add.
	Download(r Result, destDir string) (string, error)
}

// Manager fans searches out across enabled providers and merges results.
// Zero value is not usable — construct with NewManager.
type Manager struct {
	providers []Provider
	cacheDir  string
}

// NewManager builds a manager over the given providers. Pass only the
// providers that are currently enabled + configured; the manager does not
// re-check flags at search time.
func NewManager(cacheDir string, providers ...Provider) *Manager {
	return &Manager{providers: providers, cacheDir: cacheDir}
}

// HasProviders reports whether any providers are configured.
func (m *Manager) HasProviders() bool {
	return len(m.providers) > 0
}

// Providers returns the configured provider list (read-only).
func (m *Manager) Providers() []Provider {
	return m.providers
}

// Search fans out q to every configured provider in parallel, waits for all
// to finish, and returns the merged results sorted by download count
// descending (a rough proxy for quality).
func (m *Manager) Search(q Query) []Result {
	if len(m.providers) == 0 {
		return nil
	}
	type batch struct {
		items []Result
		err   error
	}
	out := make([]batch, len(m.providers))
	var wg sync.WaitGroup
	for i, p := range m.providers {
		wg.Add(1)
		go func(i int, p Provider) {
			defer wg.Done()
			items, err := p.Search(q)
			out[i] = batch{items: items, err: err}
			if err != nil {
				slog.Warn("subs provider search", "provider", p.Name(), "err", err)
			}
		}(i, p)
	}
	wg.Wait()
	var all []Result
	for _, b := range out {
		all = append(all, b.items...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Downloads > all[j].Downloads
	})
	return all
}

// Download finds the provider that produced r and invokes its Download.
// Results are written under the manager's cache directory.
func (m *Manager) Download(r Result) (string, error) {
	for _, p := range m.providers {
		if strings.EqualFold(p.Name(), r.Provider) {
			if err := os.MkdirAll(m.cacheDir, 0o755); err != nil {
				return "", fmt.Errorf("mkdir sub cache: %w", err)
			}
			return p.Download(r, m.cacheDir)
		}
	}
	return "", fmt.Errorf("no provider registered for %q", r.Provider)
}

// CacheDir returns the directory where downloaded subtitles are stored.
func (m *Manager) CacheDir() string {
	return m.cacheDir
}

// safeFilename sanitizes s for use as a filename component, replacing any
// character that isn't ASCII alphanumeric, dash, dot, or underscore with "_".
// Keeps filenames portable across Windows/Linux.
func safeFilename(s string) string {
	if s == "" {
		return "sub"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	// Trim leading dots so we don't accidentally create dotfiles
	out = strings.TrimLeft(out, ".")
	if out == "" {
		return "sub"
	}
	return out
}

// cachePath builds a deterministic path under destDir for a given provider,
// language, and identifier. Reusing the same name lets a second download of
// the same subtitle simply overwrite the cached copy.
func cachePath(destDir, provider, lang, id, ext string) string {
	if ext == "" {
		ext = ".srt"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	name := fmt.Sprintf("%s_%s_%s%s",
		safeFilename(provider), safeFilename(lang), safeFilename(id), ext)
	return filepath.Join(destDir, name)
}
