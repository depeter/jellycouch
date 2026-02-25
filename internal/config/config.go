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
	UI         UIConfig         `toml:"ui"`
	Keybinds   KeybindConfig    `toml:"keybinds"`
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

type UIConfig struct {
	Fullscreen bool `toml:"fullscreen"`
	Width      int  `toml:"width"`
	Height     int  `toml:"height"`
}

type KeybindConfig struct {
	PlayPause    string `toml:"play_pause"`
	SeekForward  string `toml:"seek_forward"`
	SeekBackward string `toml:"seek_backward"`
	SeekForwardLarge  string `toml:"seek_forward_large"`
	SeekBackwardLarge string `toml:"seek_backward_large"`
	VolumeUp     string `toml:"volume_up"`
	VolumeDown   string `toml:"volume_down"`
	Mute         string `toml:"mute"`
	SubCycle     string `toml:"sub_cycle"`
	AudioCycle   string `toml:"audio_cycle"`
	Fullscreen   string `toml:"fullscreen"`
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
		UI: UIConfig{
			Fullscreen: false,
			Width:      1920,
			Height:     1080,
		},
		Keybinds: KeybindConfig{
			PlayPause:         "Space",
			SeekForward:       "Right",
			SeekBackward:      "Left",
			SeekForwardLarge:  "Up",
			SeekBackwardLarge: "Down",
			VolumeUp:          "0",
			VolumeDown:        "9",
			Mute:              "M",
			SubCycle:          "S",
			AudioCycle:        "A",
			Fullscreen:        "F",
		},
	}
}

func ConfigDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(c)
}
