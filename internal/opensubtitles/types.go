package opensubtitles

// SubtitleResult represents a single subtitle entry from the search API.
type SubtitleResult struct {
	ID         string             `json:"id"`
	Attributes SubtitleAttributes `json:"attributes"`
}

// SubtitleAttributes holds the metadata for a subtitle result.
type SubtitleAttributes struct {
	Language        string    `json:"language"`
	DownloadCount   int       `json:"download_count"`
	ReleaseName     string    `json:"release"`
	HearingImpaired bool      `json:"hearing_impaired"`
	HD              bool      `json:"hd"`
	Files           []SubFile `json:"files"`
}

// SubFile is a downloadable file within a subtitle result.
type SubFile struct {
	FileID   int    `json:"file_id"`
	FileName string `json:"file_name"`
}

// SearchResponse is the top-level response from the subtitle search endpoint.
type SearchResponse struct {
	TotalCount int              `json:"total_count"`
	Data       []SubtitleResult `json:"data"`
}

// DownloadResponse is returned by the subtitle download-link endpoint.
type DownloadResponse struct {
	Link      string `json:"link"`
	FileName  string `json:"file_name"`
	ResetTime string `json:"reset_time"`
	Remaining int    `json:"remaining"`
}

// loginRequest is the body sent to /login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is returned by /login.
type loginResponse struct {
	Token string `json:"token"`
}

// downloadRequest is the body sent to /download.
type downloadRequest struct {
	FileID int `json:"file_id"`
}
