package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"

	"github.com/depeter/jellycouch/internal/keymap"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)

// clampInt returns v clamped to [lo, hi]. Logs a warning with the given field
// name when v is adjusted so users can spot silently-corrected config values.
func clampInt(field string, v, lo, hi int) int {
	if v < lo {
		slog.Warn("config value clamped", "field", field, "value", v, "min", lo, "max", hi, "clamped_to", lo)
		return lo
	}
	if v > hi {
		slog.Warn("config value clamped", "field", field, "value", v, "min", lo, "max", hi, "clamped_to", hi)
		return hi
	}
	return v
}

func clampFloat(field string, v, lo, hi float64) float64 {
	if v < lo {
		slog.Warn("config value clamped", "field", field, "value", v, "min", lo, "max", hi, "clamped_to", lo)
		return lo
	}
	if v > hi {
		slog.Warn("config value clamped", "field", field, "value", v, "min", lo, "max", hi, "clamped_to", hi)
		return hi
	}
	return v
}

func validateColor(field, v, fallback string) string {
	if v == "" {
		return fallback
	}
	if hexColorPattern.MatchString(v) {
		return v
	}
	slog.Warn("config color invalid, using fallback", "field", field, "value", v, "fallback", fallback)
	return fallback
}

// validate clamps out-of-range values and rejects malformed colors, logging
// warnings so misconfigured files are visible without blocking startup.
func (c *Config) validate() {
	defaults := DefaultConfig()
	c.Subtitles.FontSize = clampInt("subtitles.font_size", c.Subtitles.FontSize, 8, 200)
	c.Subtitles.Position = clampInt("subtitles.position", c.Subtitles.Position, 0, 100)
	c.Subtitles.BorderSize = clampFloat("subtitles.border_size", c.Subtitles.BorderSize, 0, 20)
	c.Subtitles.ShadowOffset = clampFloat("subtitles.shadow_offset", c.Subtitles.ShadowOffset, 0, 20)
	c.Subtitles.Delay = clampFloat("subtitles.delay", c.Subtitles.Delay, -120, 120)
	c.Subtitles.Color = validateColor("subtitles.color", c.Subtitles.Color, defaults.Subtitles.Color)
	c.Subtitles.BorderColor = validateColor("subtitles.border_color", c.Subtitles.BorderColor, defaults.Subtitles.BorderColor)
	c.Playback.Volume = clampInt("playback.volume", c.Playback.Volume, 0, 150)
	if c.Cache.MaxImageMB < 0 {
		slog.Warn("config cache.max_image_mb invalid, using 0 (unbounded)", "value", c.Cache.MaxImageMB)
		c.Cache.MaxImageMB = 0
	}
	// Treat zero as "unset" (e.g. missing from older configs) rather than a
	// literal 0.0 which would produce a divide-by-zero in Layout.
	if c.Display.UIScale == 0 {
		c.Display.UIScale = 1.0
	}
	c.Display.UIScale = clampFloat("display.ui_scale", c.Display.UIScale, 0.5, 2.0)
}

// CurrentSchemaVersion is the config format this binary writes. Migrations
// run on older values during Load. Bump when a semantic change requires a
// rewrite of user configs.
const CurrentSchemaVersion = 1

type Config struct {
	// SchemaVersion tracks format version so future migrations know where to start.
	SchemaVersion int `toml:"schema_version"`

	Server            ServerConfig            `toml:"server"`
	Jellyseerr        JellyseerrConfig        `toml:"jellyseerr"`
	Subtitles         SubtitleConfig          `toml:"subtitles"`
	SubtitleProviders SubtitleProvidersConfig `toml:"subtitle_providers"`
	Playback          PlaybackConfig          `toml:"playback"`
	Cache             CacheConfig             `toml:"cache"`
	Logging           LoggingConfig           `toml:"logging"`
	Display           DisplayConfig           `toml:"display"`
	Keybindings       keymap.Config           `toml:"keybindings"`
}

// SubtitleProvidersConfig holds credentials for each supported online
// subtitle source. A provider is only used when Enabled is true AND the
// required credentials for that provider are non-empty.
type SubtitleProvidersConfig struct {
	OpenSubtitles OpenSubtitlesConfig `toml:"opensubtitles"`
	Subdl         SubdlConfig         `toml:"subdl"`
}

type OpenSubtitlesConfig struct {
	Enabled  bool   `toml:"enabled"`
	APIKey   string `toml:"api_key"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

type SubdlConfig struct {
	Enabled bool   `toml:"enabled"`
	APIKey  string `toml:"api_key"`
}

type CacheConfig struct {
	// MaxImageMB caps the on-disk poster/thumbnail cache. 0 means unbounded.
	MaxImageMB int `toml:"max_image_mb"`
}

// DisplayConfig holds user-controlled rendering tweaks. Currently just UIScale,
// which divides the logical canvas so Ebitengine has to upscale more — making
// fonts and posters physically larger without changing any layout math.
type DisplayConfig struct {
	// UIScale multiplies the apparent size of everything on screen. 1.0 is
	// the design baseline; values above 1 make text bigger (useful from the
	// couch on a 4K TV), below 1 packs more on screen.
	UIScale float64 `toml:"ui_scale"`
}

type LoggingConfig struct {
	// Level is one of "debug", "info", "warn", "error".
	Level string `toml:"level"`
}

type WebApp struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type JellyseerrConfig struct {
	URL    string `toml:"url"`
	APIKey string `toml:"api_key"`
}

type ServerConfig struct {
	URL      string `toml:"url"`
	Username string `toml:"username"`
	Token    string `toml:"token"`
	UserID   string `toml:"user_id"`
}

type SubtitleConfig struct {
	Font        string  `toml:"font"`
	FontSize    int     `toml:"font_size"`
	Color       string  `toml:"color"`
	BorderColor string  `toml:"border_color"`
	BorderSize  float64 `toml:"border_size"`
	ShadowOffset float64 `toml:"shadow_offset"`
	Position    int     `toml:"position"`
	Delay       float64 `toml:"delay"`
	ASSOverride string  `toml:"ass_override"`
}

type PlaybackConfig struct {
	HWAccel       string `toml:"hwdec"`
	AudioLanguage string `toml:"audio_language"`
	SubLanguage   string `toml:"sub_language"`
	Volume        int    `toml:"volume"`
}

// BuiltinWebApps are always-present streaming service shortcuts.
var BuiltinWebApps = []WebApp{
	{Name: "VRT MAX", URL: "https://www.vrt.be/vrtmax/"},
	{Name: "Go Play", URL: "https://www.goplay.be"},
}

func DefaultConfig() *Config {
	return &Config{
		SchemaVersion: CurrentSchemaVersion,
		Server:        ServerConfig{},
		Subtitles: SubtitleConfig{
			Font:         "Liberation Sans",
			FontSize:     48,
			Color:        "#FFFFFF",
			BorderColor:  "#000000",
			BorderSize:   3,
			ShadowOffset: 2,
			Position:     95,
			Delay:        0,
			ASSOverride:  "force",
		},
		Playback: PlaybackConfig{
			HWAccel:       "auto-safe",
			AudioLanguage: "eng",
			SubLanguage:   "eng",
			Volume:        100,
		},
		Cache: CacheConfig{
			MaxImageMB: 500,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		Display: DisplayConfig{
			UIScale: 1.0,
		},
		Keybindings: keymap.Default(),
	}
}

func ConfigDir() (string, error) {
	// os.UserConfigDir returns:
	//   Windows: %AppData% (e.g. C:\Users\X\AppData\Roaming)
	//   Linux:   $XDG_CONFIG_HOME or ~/.config
	//   macOS:   ~/Library/Application Support
	configHome, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "jellycouch"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	path, err := ConfigPath()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.migrate()
	cfg.validate()
	return cfg, nil
}

// migrate upgrades older config formats to CurrentSchemaVersion. Each step
// should be small and idempotent — running migrate() twice must be a no-op.
func (c *Config) migrate() {
	// Fill in any absent keybinding fields with defaults, regardless of
	// schema version — this handles both brand-new and older configs that
	// predate the keybindings section.
	defaults := keymap.Default()
	if c.Keybindings.PlayPause == "" {
		c.Keybindings.PlayPause = defaults.PlayPause
	}
	if c.Keybindings.SeekSmallForward == "" {
		c.Keybindings.SeekSmallForward = defaults.SeekSmallForward
	}
	if c.Keybindings.SeekSmallBack == "" {
		c.Keybindings.SeekSmallBack = defaults.SeekSmallBack
	}
	if c.Keybindings.SeekLargeForward == "" {
		c.Keybindings.SeekLargeForward = defaults.SeekLargeForward
	}
	if c.Keybindings.SeekLargeBack == "" {
		c.Keybindings.SeekLargeBack = defaults.SeekLargeBack
	}
	if c.Keybindings.VolumeUp == "" {
		c.Keybindings.VolumeUp = defaults.VolumeUp
	}
	if c.Keybindings.VolumeDown == "" {
		c.Keybindings.VolumeDown = defaults.VolumeDown
	}
	if c.Keybindings.Mute == "" {
		c.Keybindings.Mute = defaults.Mute
	}
	if c.Keybindings.CycleSubtitles == "" {
		c.Keybindings.CycleSubtitles = defaults.CycleSubtitles
	}
	if c.Keybindings.CycleAudio == "" {
		c.Keybindings.CycleAudio = defaults.CycleAudio
	}
	if c.Keybindings.ShowInfo == "" {
		c.Keybindings.ShowInfo = defaults.ShowInfo
	}

	if c.SchemaVersion == CurrentSchemaVersion {
		return
	}
	if c.SchemaVersion == 0 {
		// v0 → v1: configs written before versioning; no field renames yet,
		// so upgrading is just stamping the version.
		slog.Info("config migrating", "from", 0, "to", 1)
		c.SchemaVersion = 1
	}
	// Future migrations append here as `if c.SchemaVersion == 1 { ... c.SchemaVersion = 2 }`.
	if c.SchemaVersion != CurrentSchemaVersion {
		slog.Warn("config schema newer than this binary understands; leaving as-is",
			"config_version", c.SchemaVersion, "binary_version", CurrentSchemaVersion)
	}
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Write to a temp file first and atomically rename, so a crash
	// mid-write won't leave a truncated/corrupt config file.
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if err := toml.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
