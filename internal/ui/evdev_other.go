//go:build !linux

package ui

// evdevBackJustPressed is a no-op on non-Linux platforms.
func evdevBackJustPressed() bool {
	return false
}
