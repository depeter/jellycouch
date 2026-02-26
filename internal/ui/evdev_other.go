//go:build !linux

package ui

// EvdevBackJustPressed is a no-op on non-Linux platforms.
func EvdevBackJustPressed() bool {
	return false
}

// EvdevRecentEvents is a no-op on non-Linux platforms.
func EvdevRecentEvents() []EvdevEvent {
	return nil
}
