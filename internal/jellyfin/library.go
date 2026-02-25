package jellyfin

import (
	"fmt"
	"sort"

	jellyfin "github.com/sj14/jellyfin-go/api"
)

// LibraryFilter holds server-side filter/sort parameters for library queries.
type LibraryFilter struct {
	SortBy    string   // SDK ItemSortBy string value
	SortOrder string   // "Ascending" / "Descending"
	Genres    []string // genre names (empty = all)
	Status    string   // ItemFilter value: "", "IsPlayed", "IsUnplayed", "IsFavorite", "IsResumable"
	Search    string   // text search term
	Letter    string   // NameStartsWith filter (single letter or "#" for non-alpha)
	Years     []int32  // filter by production years
}

// MediaItem is a simplified representation of a Jellyfin item.
type MediaItem struct {
	ID                    string
	Name                  string
	Type                  string // Movie, Series, Episode, Season, etc.
	Year                  int
	Overview              string
	RuntimeTicks          int64
	CommunityRating       float32
	ImageTags             map[string]string
	BackdropTags          []string
	SeriesID              string
	SeriesName            string
	SeasonID              string
	SeasonName            string
	IndexNumber           int
	ParentIndexNumber     int
	Played                bool
	PlaybackPositionTicks int64
	UserData              *UserData
}

type UserData struct {
	PlaybackPositionTicks int64
	PlayCount             int
	Played                bool
	IsFavorite            bool
}

// GetViews returns the user's media libraries (Movies, TV Shows, Music, etc.)
func (c *Client) GetViews() ([]MediaItem, error) {
	result, _, err := c.api.UserViewsAPI.GetUserViews(c.ctx).UserId(c.userID).Execute()
	if err != nil {
		return nil, fmt.Errorf("get views: %w", err)
	}
	return convertItems(result.Items), nil
}

// GetLatestMedia returns the latest items in a library.
func (c *Client) GetLatestMedia(parentID string, limit int) ([]MediaItem, error) {
	req := c.api.UserLibraryAPI.GetLatestMedia(c.ctx).
		UserId(c.userID).
		Limit(int32(limit)).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW, jellyfin.ITEMFIELDS_PRIMARY_IMAGE_ASPECT_RATIO}).
		EnableImageTypes([]jellyfin.ImageType{jellyfin.IMAGETYPE_PRIMARY, jellyfin.IMAGETYPE_BACKDROP}).
		ImageTypeLimit(1)
	if parentID != "" {
		req = req.ParentId(parentID)
	}
	items, _, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("get latest: %w", err)
	}
	return convertBaseItemDtoArray(items), nil
}

// GetItems returns items in a library with pagination.
func (c *Client) GetItems(parentID string, start, limit int, itemTypes []string) ([]MediaItem, int, error) {
	req := c.api.ItemsAPI.GetItems(c.ctx).
		UserId(c.userID).
		StartIndex(int32(start)).
		Limit(int32(limit)).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW, jellyfin.ITEMFIELDS_PRIMARY_IMAGE_ASPECT_RATIO}).
		EnableImageTypes([]jellyfin.ImageType{jellyfin.IMAGETYPE_PRIMARY, jellyfin.IMAGETYPE_BACKDROP}).
		ImageTypeLimit(1).
		Recursive(true).
		SortBy([]jellyfin.ItemSortBy{jellyfin.ITEMSORTBY_SORT_NAME}).
		SortOrder([]jellyfin.SortOrder{jellyfin.SORTORDER_ASCENDING})
	if parentID != "" {
		req = req.ParentId(parentID)
	}
	if len(itemTypes) > 0 {
		baseTypes := make([]jellyfin.BaseItemKind, len(itemTypes))
		for i, t := range itemTypes {
			baseTypes[i] = jellyfin.BaseItemKind(t)
		}
		req = req.IncludeItemTypes(baseTypes)
	}
	result, _, err := req.Execute()
	if err != nil {
		return nil, 0, fmt.Errorf("get items: %w", err)
	}
	total := 0
	if result.TotalRecordCount != nil {
		total = int(*result.TotalRecordCount)
	}
	return convertItems(result.Items), total, nil
}

// GetFilteredItems returns items in a library with pagination and filtering.
func (c *Client) GetFilteredItems(parentID string, start, limit int, itemTypes []string, filter LibraryFilter) ([]MediaItem, int, error) {
	sortBy := jellyfin.ItemSortBy(filter.SortBy)
	if filter.SortBy == "" {
		sortBy = jellyfin.ITEMSORTBY_SORT_NAME
	}
	sortOrder := jellyfin.SortOrder(filter.SortOrder)
	if filter.SortOrder == "" {
		sortOrder = jellyfin.SORTORDER_ASCENDING
	}

	req := c.api.ItemsAPI.GetItems(c.ctx).
		UserId(c.userID).
		StartIndex(int32(start)).
		Limit(int32(limit)).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_PRIMARY_IMAGE_ASPECT_RATIO}).
		EnableImageTypes([]jellyfin.ImageType{jellyfin.IMAGETYPE_PRIMARY}).
		ImageTypeLimit(1).
		Recursive(true).
		SortBy([]jellyfin.ItemSortBy{sortBy}).
		SortOrder([]jellyfin.SortOrder{sortOrder})

	if parentID != "" {
		req = req.ParentId(parentID)
	}
	if len(itemTypes) > 0 {
		baseTypes := make([]jellyfin.BaseItemKind, len(itemTypes))
		for i, t := range itemTypes {
			baseTypes[i] = jellyfin.BaseItemKind(t)
		}
		req = req.IncludeItemTypes(baseTypes)
	}
	if len(filter.Genres) > 0 {
		req = req.Genres(filter.Genres)
	}
	if filter.Status != "" {
		req = req.Filters([]jellyfin.ItemFilter{jellyfin.ItemFilter(filter.Status)})
	}
	if filter.Search != "" {
		req = req.SearchTerm(filter.Search)
	}
	if filter.Letter != "" {
		req = req.NameStartsWith(filter.Letter)
	}
	if len(filter.Years) > 0 {
		req = req.Years(filter.Years)
	}

	result, _, err := req.Execute()
	if err != nil {
		return nil, 0, fmt.Errorf("get filtered items: %w", err)
	}
	total := 0
	if result.TotalRecordCount != nil {
		total = int(*result.TotalRecordCount)
	}
	return convertItems(result.Items), total, nil
}

// GetGenres returns genre names for a library, sorted alphabetically.
func (c *Client) GetGenres(parentID string, itemTypes []string) ([]string, error) {
	req := c.api.GenresAPI.GetGenres(c.ctx).
		UserId(c.userID)
	if parentID != "" {
		req = req.ParentId(parentID)
	}
	if len(itemTypes) > 0 {
		baseTypes := make([]jellyfin.BaseItemKind, len(itemTypes))
		for i, t := range itemTypes {
			baseTypes[i] = jellyfin.BaseItemKind(t)
		}
		req = req.IncludeItemTypes(baseTypes)
	}
	result, _, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("get genres: %w", err)
	}
	var genres []string
	for _, item := range result.Items {
		genres = append(genres, item.GetName())
	}
	sort.Strings(genres)
	return genres, nil
}

// GetSeasons returns seasons for a series.
func (c *Client) GetSeasons(seriesID string) ([]MediaItem, error) {
	result, _, err := c.api.TvShowsAPI.GetSeasons(c.ctx, seriesID).
		UserId(c.userID).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("get seasons: %w", err)
	}
	return convertItems(result.Items), nil
}

// GetEpisodes returns episodes for a season.
func (c *Client) GetEpisodes(seriesID string, seasonID string) ([]MediaItem, error) {
	req := c.api.TvShowsAPI.GetEpisodes(c.ctx, seriesID).
		UserId(c.userID).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW}).
		SeasonId(seasonID)
	result, _, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("get episodes: %w", err)
	}
	return convertItems(result.Items), nil
}

// GetResumeItems returns items the user can resume watching.
func (c *Client) GetResumeItems(limit int) ([]MediaItem, error) {
	result, _, err := c.api.ItemsAPI.GetResumeItems(c.ctx).
		UserId(c.userID).
		Limit(int32(limit)).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW}).
		EnableImageTypes([]jellyfin.ImageType{jellyfin.IMAGETYPE_PRIMARY, jellyfin.IMAGETYPE_BACKDROP}).
		ImageTypeLimit(1).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("get resume: %w", err)
	}
	return convertItems(result.Items), nil
}

// GetNextUp returns next episodes to watch.
func (c *Client) GetNextUp(limit int) ([]MediaItem, error) {
	result, _, err := c.api.TvShowsAPI.GetNextUp(c.ctx).
		UserId(c.userID).
		Limit(int32(limit)).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW}).
		EnableImageTypes([]jellyfin.ImageType{jellyfin.IMAGETYPE_PRIMARY}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("get next up: %w", err)
	}
	return convertItems(result.Items), nil
}

// SearchItems searches for items by name.
func (c *Client) SearchItems(query string, limit int) ([]MediaItem, error) {
	result, _, err := c.api.ItemsAPI.GetItems(c.ctx).
		UserId(c.userID).
		SearchTerm(query).
		Limit(int32(limit)).
		Recursive(true).
		Fields([]jellyfin.ItemFields{jellyfin.ITEMFIELDS_OVERVIEW}).
		EnableImageTypes([]jellyfin.ImageType{jellyfin.IMAGETYPE_PRIMARY}).
		IncludeItemTypes([]jellyfin.BaseItemKind{
			jellyfin.BASEITEMKIND_MOVIE,
			jellyfin.BASEITEMKIND_SERIES,
			jellyfin.BASEITEMKIND_EPISODE,
		}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return convertItems(result.Items), nil
}

// GetItem returns a single item by ID.
func (c *Client) GetItem(itemID string) (*MediaItem, error) {
	result, _, err := c.api.UserLibraryAPI.GetItem(c.ctx, itemID).
		UserId(c.userID).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	item := convertBaseItemDto(result)
	return &item, nil
}

func convertItems(items []jellyfin.BaseItemDto) []MediaItem {
	result := make([]MediaItem, 0, len(items))
	for _, item := range items {
		result = append(result, convertBaseItemDto(&item))
	}
	return result
}

func convertBaseItemDtoArray(items []jellyfin.BaseItemDto) []MediaItem {
	return convertItems(items)
}

func convertBaseItemDto(item *jellyfin.BaseItemDto) MediaItem {
	mi := MediaItem{}
	if item.Id != nil {
		mi.ID = *item.Id
	}
	mi.Name = item.GetName()
	if item.Type != nil {
		mi.Type = string(*item.Type)
	}
	mi.Year = int(item.GetProductionYear())
	mi.Overview = item.GetOverview()
	mi.RuntimeTicks = item.GetRunTimeTicks()
	mi.CommunityRating = item.GetCommunityRating()

	if len(item.ImageTags) > 0 {
		mi.ImageTags = make(map[string]string)
		for k, v := range item.ImageTags {
			mi.ImageTags[k] = v
		}
	}
	mi.BackdropTags = item.BackdropImageTags
	mi.SeriesID = item.GetSeriesId()
	mi.SeriesName = item.GetSeriesName()
	mi.SeasonID = item.GetSeasonId()
	mi.SeasonName = item.GetSeasonName()
	mi.IndexNumber = int(item.GetIndexNumber())
	mi.ParentIndexNumber = int(item.GetParentIndexNumber())

	if item.UserData.IsSet() {
		udPtr := item.UserData.Get()
		if udPtr != nil {
			ud := &UserData{}
			ud.PlaybackPositionTicks = udPtr.GetPlaybackPositionTicks()
			mi.PlaybackPositionTicks = ud.PlaybackPositionTicks
			ud.PlayCount = int(udPtr.GetPlayCount())
			ud.Played = udPtr.GetPlayed()
			mi.Played = ud.Played
			ud.IsFavorite = udPtr.GetIsFavorite()
			mi.UserData = ud
		}
	}
	return mi
}
