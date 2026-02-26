package jellyfin

import (
	"fmt"
	"time"

	jellyfin "github.com/sj14/jellyfin-go/api"
)

// ReportPlaybackStart notifies the server that playback has started.
func (c *Client) ReportPlaybackStart(itemID string, positionTicks int64) error {
	body := *jellyfin.NewPlaybackStartInfo()
	body.SetItemId(itemID)
	body.SetPositionTicks(positionTicks)
	body.SetCanSeek(true)
	body.SetPlayMethod(jellyfin.PLAYMETHOD_DIRECT_PLAY)

	_, err := c.api.PlaystateAPI.ReportPlaybackStart(c.reqCtx()).PlaybackStartInfo(body).Execute()
	if err != nil {
		return fmt.Errorf("report playback start: %w", err)
	}
	return nil
}

// ReportPlaybackProgress sends a progress update to the server.
func (c *Client) ReportPlaybackProgress(itemID string, positionTicks int64, isPaused bool) error {
	body := *jellyfin.NewPlaybackProgressInfo()
	body.SetItemId(itemID)
	body.SetPositionTicks(positionTicks)
	body.SetIsPaused(isPaused)
	body.SetCanSeek(true)
	body.SetPlayMethod(jellyfin.PLAYMETHOD_DIRECT_PLAY)

	_, err := c.api.PlaystateAPI.ReportPlaybackProgress(c.reqCtx()).PlaybackProgressInfo(body).Execute()
	if err != nil {
		return fmt.Errorf("report progress: %w", err)
	}
	return nil
}

// ReportPlaybackStopped notifies the server that playback has stopped.
func (c *Client) ReportPlaybackStopped(itemID string, positionTicks int64) error {
	body := *jellyfin.NewPlaybackStopInfo()
	body.SetItemId(itemID)
	body.SetPositionTicks(positionTicks)

	_, err := c.api.PlaystateAPI.ReportPlaybackStopped(c.reqCtx()).PlaybackStopInfo(body).Execute()
	if err != nil {
		return fmt.Errorf("report playback stopped: %w", err)
	}
	return nil
}

// MarkPlayed marks an item as played.
func (c *Client) MarkPlayed(itemID string) error {
	_, _, err := c.api.PlaystateAPI.MarkPlayedItem(c.ctx, itemID).
		UserId(c.userID).
		DatePlayed(time.Now()).
		Execute()
	if err != nil {
		return fmt.Errorf("mark played: %w", err)
	}
	return nil
}

// MarkUnplayed marks an item as unplayed.
func (c *Client) MarkUnplayed(itemID string) error {
	_, _, err := c.api.PlaystateAPI.MarkUnplayedItem(c.ctx, itemID).
		UserId(c.userID).
		Execute()
	if err != nil {
		return fmt.Errorf("mark unplayed: %w", err)
	}
	return nil
}
