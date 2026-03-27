package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server     ServerConfig     `toml:"server"`
	Jellyseerr JellyseerrConfig `toml:"jellyseerr"`
	Subtitles  SubtitleConfig   `toml:"subtitles"`
	Playback   PlaybackConfig   `toml:"playback"`
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
		Server: ServerConfig{},
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
	return cfg, nil
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
