package player

import (
	"fmt"

	"github.com/gen2brain/go-mpv"

	"github.com/depeter/jellycouch/internal/config"
)

// ApplySubtitleConfig applies subtitle settings from config to the mpv instance.
func (p *Player) ApplySubtitleConfig(cfg *config.SubtitleConfig) {
	p.do(func(m *mpv.Mpv) error {
		m.SetPropertyString("sub-font", cfg.Font)
		m.SetPropertyString("sub-font-size", fmt.Sprintf("%d", cfg.FontSize))
		m.SetPropertyString("sub-color", cfg.Color)
		m.SetPropertyString("sub-border-color", cfg.BorderColor)
		m.SetPropertyString("sub-border-size", fmt.Sprintf("%.1f", cfg.BorderSize))
		m.SetPropertyString("sub-shadow-offset", fmt.Sprintf("%.1f", cfg.ShadowOffset))
		m.SetPropertyString("sub-pos", fmt.Sprintf("%d", cfg.Position))
		if cfg.Delay != 0 {
			m.SetPropertyString("sub-delay", fmt.Sprintf("%.3f", cfg.Delay))
		}
		if cfg.ASSOverride != "" {
			m.SetPropertyString("sub-ass-override", cfg.ASSOverride)
		}
		return nil
	})
}

// SetSubDelay adjusts subtitle delay in seconds.
func (p *Player) SetSubDelay(seconds float64) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.SetPropertyString("sub-delay", fmt.Sprintf("%.3f", seconds))
	})
}

// SetSecondarySubtitle enables a secondary subtitle track.
func (p *Player) SetSecondarySubtitle(trackID int) error {
	return p.do(func(m *mpv.Mpv) error {
		return m.SetPropertyString("secondary-sid", fmt.Sprintf("%d", trackID))
	})
}
