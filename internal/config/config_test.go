package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.validate()
	if cfg.Playback.Volume < 0 || cfg.Playback.Volume > 150 {
		t.Errorf("default volume out of range: %d", cfg.Playback.Volume)
	}
	if cfg.Subtitles.Position < 0 || cfg.Subtitles.Position > 100 {
		t.Errorf("default position out of range: %d", cfg.Subtitles.Position)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("default schema version = %d, want %d", cfg.SchemaVersion, CurrentSchemaVersion)
	}
}

func TestValidateClamps(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Playback.Volume = -50
	cfg.Subtitles.Position = 500
	cfg.Subtitles.FontSize = 2
	cfg.Subtitles.BorderSize = -1
	cfg.Cache.MaxImageMB = -10
	cfg.validate()

	if cfg.Playback.Volume != 0 {
		t.Errorf("volume not clamped: %d", cfg.Playback.Volume)
	}
	if cfg.Subtitles.Position != 100 {
		t.Errorf("position not clamped: %d", cfg.Subtitles.Position)
	}
	if cfg.Subtitles.FontSize != 8 {
		t.Errorf("font size not clamped: %d", cfg.Subtitles.FontSize)
	}
	if cfg.Subtitles.BorderSize != 0 {
		t.Errorf("border size not clamped: %g", cfg.Subtitles.BorderSize)
	}
	if cfg.Cache.MaxImageMB != 0 {
		t.Errorf("max image mb not coerced: %d", cfg.Cache.MaxImageMB)
	}
}

func TestValidateUIScale(t *testing.T) {
	// Missing from TOML → zero float → must be coerced to 1.0, not left at 0
	// (which would divide by zero in Layout).
	cfg := DefaultConfig()
	cfg.Display.UIScale = 0
	cfg.validate()
	if cfg.Display.UIScale != 1.0 {
		t.Errorf("zero ui_scale not coerced to 1.0: %g", cfg.Display.UIScale)
	}

	// Out of range on both ends gets clamped.
	cfg = DefaultConfig()
	cfg.Display.UIScale = 10
	cfg.validate()
	if cfg.Display.UIScale != 2.0 {
		t.Errorf("high ui_scale not clamped to 2.0: %g", cfg.Display.UIScale)
	}

	cfg = DefaultConfig()
	cfg.Display.UIScale = 0.1
	cfg.validate()
	if cfg.Display.UIScale != 0.5 {
		t.Errorf("low ui_scale not clamped to 0.5: %g", cfg.Display.UIScale)
	}
}

func TestValidateRejectsBadColor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Subtitles.Color = "not-a-color"
	cfg.validate()
	if cfg.Subtitles.Color != DefaultConfig().Subtitles.Color {
		t.Errorf("bad color not replaced with default: %q", cfg.Subtitles.Color)
	}
}

func TestMigrateFromZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SchemaVersion = 0
	cfg.migrate()
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("migrate did not advance schema: %d", cfg.SchemaVersion)
	}
}

// TestRoundTrip writes the default config via toml, reads it back, and
// verifies key fields survive encode/decode.
func TestRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.URL = "https://example.com"
	cfg.Server.Username = "alice"
	cfg.Playback.Volume = 75

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.toml")

	f, err := os.Create(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Config
	if err := toml.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Server.URL != "https://example.com" {
		t.Errorf("server.url lost: %q", loaded.Server.URL)
	}
	if loaded.Server.Username != "alice" {
		t.Errorf("server.username lost: %q", loaded.Server.Username)
	}
	if loaded.Playback.Volume != 75 {
		t.Errorf("playback.volume lost: %d", loaded.Playback.Volume)
	}
	if loaded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("schema_version lost: %d", loaded.SchemaVersion)
	}
}
