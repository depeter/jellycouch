package cache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// ImageCache provides disk + memory caching for images.
type ImageCache struct {
	cacheDir     string
	memory       sync.Map // url -> *ebiten.Image
	loading      sync.Map // url -> *loadEntry (in-flight dedup with waiters)
	sem          chan struct{}
	maxDiskBytes int64        // 0 = unbounded
	diskBytes    atomic.Int64 // best-effort running total of on-disk cache size
	evictMu      sync.Mutex   // serialize eviction passes
}

// loadEntry tracks in-flight downloads and their waiters.
type loadEntry struct {
	mu        sync.Mutex
	callbacks []func(*ebiten.Image)
}

// NewImageCache creates a new image cache with the given disk directory.
// maxDiskMB caps the on-disk footprint; pass 0 for unbounded.
func NewImageCache(cacheDir string, maxDiskMB int) (*ImageCache, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	ic := &ImageCache{
		cacheDir:     cacheDir,
		sem:          make(chan struct{}, 6),
		maxDiskBytes: int64(maxDiskMB) * 1024 * 1024,
	}
	ic.diskBytes.Store(ic.scanDiskSize())
	if ic.maxDiskBytes > 0 && ic.diskBytes.Load() > ic.maxDiskBytes {
		go ic.evictIfNeeded()
	}
	return ic, nil
}

// scanDiskSize walks the cache dir and returns the sum of regular-file sizes.
func (ic *ImageCache) scanDiskSize() int64 {
	var total int64
	filepath.Walk(ic.cacheDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// evictIfNeeded trims the cache to 80% of the cap by deleting oldest files
// (by mtime). Safe to call concurrently; only one eviction pass runs at a time.
func (ic *ImageCache) evictIfNeeded() {
	if ic.maxDiskBytes <= 0 {
		return
	}
	if !ic.evictMu.TryLock() {
		return
	}
	defer ic.evictMu.Unlock()

	total := ic.diskBytes.Load()
	if total <= ic.maxDiskBytes {
		return
	}

	type entry struct {
		path  string
		size  int64
		mtime time.Time
	}
	var entries []entry
	filepath.Walk(ic.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		entries = append(entries, entry{path: path, size: info.Size(), mtime: info.ModTime()})
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].mtime.Before(entries[j].mtime) })

	target := ic.maxDiskBytes * 80 / 100
	var freed int64
	for _, e := range entries {
		if total-freed <= target {
			break
		}
		if err := os.Remove(e.path); err == nil {
			freed += e.size
		}
	}
	if freed > 0 {
		ic.diskBytes.Add(-freed)
		slog.Info("image cache evicted", "freed_bytes", freed, "size_bytes", ic.diskBytes.Load(), "max_bytes", ic.maxDiskBytes)
	}
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
		// Acquire semaphore to limit concurrent downloads
		ic.sem <- struct{}{}
		defer func() { <-ic.sem }()

		img, err := ic.loadImage(url)
		if err != nil {
			slog.Warn("image load failed", "url", url, "err", err)
			ic.loading.Delete(url)
			// Notify all waiters with nil so they don't hang forever
			entry.mu.Lock()
			cbs := make([]func(*ebiten.Image), len(entry.callbacks))
			copy(cbs, entry.callbacks)
			entry.mu.Unlock()
			for _, cb := range cbs {
				cb(nil)
			}
			return
		}

		eimg := ebiten.NewImageFromImage(img)
		// Store in memory before removing from loading map to prevent
		// a race where a concurrent LoadAsync misses both caches and
		// starts a duplicate download.
		ic.memory.Store(url, eimg)
		ic.loading.Delete(url)

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
			// Touch mtime so LRU eviction keeps recently-used files.
			now := time.Now()
			os.Chtimes(diskPath, now, now)
			return img, nil
		}
		// Corrupt cache file, remove and re-download
		if info, statErr := os.Stat(diskPath); statErr == nil {
			ic.diskBytes.Add(-info.Size())
		}
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

	// Buffer the full response in memory, then decode. Only if decode
	// succeeds do we persist the bytes to disk. This prevents a truncated
	// download from leaving a partial file that future loads would accept
	// (tolerant JPEG decoders can succeed on truncated input).
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Decode succeeded — persist.
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(diskPath, data, 0o644); err != nil {
		// In-memory image is still valid; just report the disk error.
		return img, nil
	}
	ic.diskBytes.Add(int64(len(data)))
	if ic.maxDiskBytes > 0 && ic.diskBytes.Load() > ic.maxDiskBytes {
		go ic.evictIfNeeded()
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
	ic.memory.Range(func(key, value any) bool {
		ic.memory.Delete(key)
		return true
	})
}

// ClearDisk removes all cached images from disk.
func (ic *ImageCache) ClearDisk() error {
	return os.RemoveAll(ic.cacheDir)
}
