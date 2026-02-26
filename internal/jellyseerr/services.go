package jellyseerr

import "fmt"

// GetRadarrSettings returns all configured Radarr servers from Jellyseerr.
func (c *Client) GetRadarrSettings() ([]RadarrSettings, error) {
	var settings []RadarrSettings
	if err := c.get(pathRadarr, &settings); err != nil {
		return nil, fmt.Errorf("get radarr settings: %w", err)
	}
	return settings, nil
}

// GetSonarrSettings returns all configured Sonarr servers from Jellyseerr.
func (c *Client) GetSonarrSettings() ([]SonarrSettings, error) {
	var settings []SonarrSettings
	if err := c.get(pathSonarr, &settings); err != nil {
		return nil, fmt.Errorf("get sonarr settings: %w", err)
	}
	return settings, nil
}
