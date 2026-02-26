package cache

import (
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// ImageCache provides disk + memory caching for images.
type ImageCache struct {
	cacheDir string
	memory   sync.Map // url -> *ebiten.Image
	loading  sync.Map // url -> *loadEntry (in-flight dedup with waiters)
	sem      chan struct{}
}

// loadEntry tracks in-flight downloads and their waiters.
type loadEntry struct {
	mu        sync.Mutex
	callbacks []func(*ebiten.Image)
}

// NewImageCache creates a new image cache with the given disk directory.
func NewImageCache(cacheDir string) (*ImageCache, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	return &ImageCache{
		cacheDir: cacheDir,
		sem:      make(chan struct{}, 6),
	}, nil
}

// Get returns a cached image if available, or nil.
func (ic *ImageCache) Get(url string) *ebiten.Image {
	if v, ok := ic.memory.Load(url); ok {
		return v.(*ebiten.Image)
	}
	return nil
}

// LoadAsync starts loading an image from URL in the background.
// The callback is called with the image when ready (may be called from a goroutine).
func (ic *ImageCache) LoadAsync(url string, callback func(*ebiten.Image)) {
	// Already in memory?
	if v, ok := ic.memory.Load(url); ok {
		callback(v.(*ebiten.Image))
		return
	}

	// Dedup in-flight requests — add callback to existing entry or create new one
	entry := &loadEntry{}
	entry.callbacks = append(entry.callbacks, callback)

	if existing, loaded := ic.loading.LoadOrStore(url, entry); loaded {
		// Another goroutine is already downloading this URL — append our callback
		existingEntry := existing.(*loadEntry)
		existingEntry.mu.Lock()
		existingEntry.callbacks = append(existingEntry.callbacks, callback)
		existingEntry.mu.Unlock()
		return
	}

	go func() {
		defer ic.loading.Delete(url)

		// Acquire semaphore to limit concurrent downloads
		ic.sem <- struct{}{}
		defer func() { <-ic.sem }()

		img, err := ic.loadImage(url)
		if err != nil {
			return
		}

		eimg := ebiten.NewImageFromImage(img)
		ic.memory.Store(url, eimg)

		// Notify all waiters
		entry.mu.Lock()
		cbs := make([]func(*ebiten.Image), len(entry.callbacks))
		copy(cbs, entry.callbacks)
		entry.mu.Unlock()

		for _, cb := range cbs {
			cb(eimg)
		}
	}()
}

func (ic *ImageCache) loadImage(url string) (image.Image, error) {
	diskPath := ic.diskPath(url)

	// Try disk cache first
	if f, err := os.Open(diskPath); err == nil {
		defer f.Close()
		img, _, err := image.Decode(f)
		if err == nil {
			return img, nil
		}
		// Corrupt cache file, remove and re-download
		os.Remove(diskPath)
	}

	// Download with timeout-aware client
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download failed: %s", resp.Status)
	}

	// Save to disk
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(diskPath)
	if err != nil {
		return nil, err
	}

	// Tee to disk while decoding
	tee := io.TeeReader(resp.Body, f)
	img, _, err := image.Decode(tee)
	f.Close()
	if err != nil {
		os.Remove(diskPath)
		return nil, err
	}

	return img, nil
}

func (ic *ImageCache) diskPath(url string) string {
	h := sha256.Sum256([]byte(url))
	name := fmt.Sprintf("%x", h[:16])
	return filepath.Join(ic.cacheDir, name[:2], name)
}

// LoadDecodedImage downloads and decodes an image from URL, returning a
// standard image.Image. Uses the same disk cache as LoadAsync but does not
// store the result in the in-memory ebiten cache.
func (ic *ImageCache) LoadDecodedImage(url string) (image.Image, error) {
	return ic.loadImage(url)
}

// CacheDir returns the disk cache directory path.
func (ic *ImageCache) CacheDir() string {
	return ic.cacheDir
}

// Clear removes all cached images from memory.
func (ic *ImageCache) Clear() {
	ic.memory = sync.Map{}
}

// ClearDisk removes all cached images from disk.
func (ic *ImageCache) ClearDisk() error {
	return os.RemoveAll(ic.cacheDir)
}
