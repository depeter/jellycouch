package jellyfin

import "testing"

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"jellyfin.example.com", "https://jellyfin.example.com"},
		{"https://jellyfin.example.com", "https://jellyfin.example.com"},
		{"https://jellyfin.example.com/", "https://jellyfin.example.com"},
		{"https://jellyfin.example.com/////", "https://jellyfin.example.com"},
		{"http://localhost:8096", "http://localhost:8096"},
		{"   jellyfin.example.com   ", "https://jellyfin.example.com"},
		{"https://jellyfin.example.com/path/", "https://jellyfin.example.com/path"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := normalizeURL(tt.in)
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
