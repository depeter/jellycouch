package cache

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskPathIsDeterministic(t *testing.T) {
	ic := &ImageCache{cacheDir: "/tmp/x"}
	a := ic.diskPath("https://example.com/image.jpg")
	b := ic.diskPath("https://example.com/image.jpg")
	if a != b {
		t.Errorf("non-deterministic hash: %q vs %q", a, b)
	}
}

func TestDiskPathDifferentURLs(t *testing.T) {
	ic := &ImageCache{cacheDir: "/tmp/x"}
	a := ic.diskPath("https://example.com/a.jpg")
	b := ic.diskPath("https://example.com/b.jpg")
	if a == b {
		t.Errorf("collision for distinct urls: %q", a)
	}
}

func TestDiskPathInsideCacheDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "imgs")
	ic := &ImageCache{cacheDir: dir}
	p := ic.diskPath("https://example.com/poster.jpg")
	if !strings.HasPrefix(p, dir+string(filepath.Separator)) {
		t.Errorf("path %q not under cache dir %q", p, dir)
	}
}

func TestScanDiskSize(t *testing.T) {
	tmpDir := t.TempDir()
	ic, err := NewImageCache(tmpDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := ic.scanDiskSize(); got != 0 {
		t.Errorf("empty cache size = %d, want 0", got)
	}

	// Drop a file and confirm size updates.
	f := filepath.Join(tmpDir, "ab", "abcdef")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ic.scanDiskSize(); got != 5 {
		t.Errorf("scanDiskSize = %d, want 5", got)
	}
}

// TestLoadImageUndecodableNotPersisted verifies that a bogus payload fails
// to decode and crucially leaves nothing on disk — proving the
// buffer-then-write guard.
func TestLoadImageUndecodableNotPersisted(t *testing.T) {
	tmpDir := t.TempDir()
	ic, err := NewImageCache(tmpDir, 0)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-an-image"))
	}))
	defer srv.Close()

	_, err = ic.loadImage(srv.URL + "/poster.jpg")
	if err == nil {
		t.Fatal("expected decode to fail on garbage payload")
	}
	if got := ic.scanDiskSize(); got != 0 {
		t.Errorf("disk should be empty after failed decode, got %d bytes", got)
	}
	if got := ic.diskBytes.Load(); got != 0 {
		t.Errorf("diskBytes counter should be 0, got %d", got)
	}
}
