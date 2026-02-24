package player

import (
	"fmt"

	"github.com/depeter/jellycouch/internal/config"
)

// ApplySubtitleConfig applies subtitle settings from config to the mpv instance.
func (p *Player) ApplySubtitleConfig(cfg *config.SubtitleConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.m.SetPropertyString("sub-font", cfg.Font)
	p.m.SetPropertyString("sub-font-size", fmt.Sprintf("%d", cfg.FontSize))
	p.m.SetPropertyString("sub-color", cfg.Color)
	p.m.SetPropertyString("sub-border-color", cfg.BorderColor)
	p.m.SetPropertyString("sub-border-size", fmt.Sprintf("%.1f", cfg.BorderSize))
	p.m.SetPropertyString("sub-shadow-offset", fmt.Sprintf("%.1f", cfg.ShadowOffset))
	p.m.SetPropertyString("sub-pos", fmt.Sprintf("%d", cfg.Position))
	if cfg.Delay != 0 {
		p.m.SetPropertyString("sub-delay", fmt.Sprintf("%.3f", cfg.Delay))
	}
	if cfg.ASSOverride != "" {
		p.m.SetPropertyString("sub-ass-override", cfg.ASSOverride)
	}
}

// SetSubDelay adjusts subtitle delay in seconds.
func (p *Player) SetSubDelay(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.SetPropertyString("sub-delay", fmt.Sprintf("%.3f", seconds))
}

// SetSecondarySubtitle enables a secondary subtitle track.
func (p *Player) SetSecondarySubtitle(trackID int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.SetPropertyString("secondary-sid", fmt.Sprintf("%d", trackID))
}
