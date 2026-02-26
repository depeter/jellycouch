package ui

import "time"

// EvdevEvent represents a captured evdev input event.
type EvdevEvent struct {
	Time   time.Time
	Device string // e.g. "event3"
	Type   uint16
	Code   uint16
	Value  int32
}
