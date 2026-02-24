package app

// AppState represents the top-level application mode.
type AppState int

const (
	StateBrowse AppState = iota
	StatePlay
)
