//go:build linux

package ui

import (
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// Linux evdev constants
const (
	evKey   = 0x01
	keyBack = 158 // KEY_BACK (XF86Back)
)

// inputEventSize is the size of a Linux input_event struct (timeval + u16 + u16 + s32).
var inputEventSize = int(unsafe.Sizeof(struct {
	Sec, Usec int64
	Type      uint16
	Code      uint16
	Value     int32
}{}))

var evdevBackPressed atomic.Bool

const recentEventsMax = 8

var (
	recentEventsMu sync.Mutex
	recentEvents   []EvdevEvent
)

func init() {
	go watchEvdevBack()
}

// watchEvdevBack scans /dev/input/event* devices for KEY_BACK press events.
func watchEvdevBack() {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(matches) == 0 {
		return
	}

	for _, path := range matches {
		go readEvdev(path)
	}
}

func readEvdev(path string) {
	f, err := os.Open(path)
	if err != nil {
		// No permission or device not accessible — skip silently
		return
	}
	defer f.Close()

	device := filepath.Base(path)
	buf := make([]byte, inputEventSize)
	for {
		_, err := f.Read(buf)
		if err != nil {
			return
		}

		// Parse type (offset 16), code (offset 18), value (offset 20)
		typ := binary.LittleEndian.Uint16(buf[16:18])
		code := binary.LittleEndian.Uint16(buf[18:20])
		value := int32(binary.LittleEndian.Uint32(buf[20:24]))

		// Record all key-press events (value=1) into ring buffer
		if typ == evKey && value == 1 {
			ev := EvdevEvent{
				Time:   time.Now(),
				Device: device,
				Type:   typ,
				Code:   code,
				Value:  value,
			}
			recentEventsMu.Lock()
			recentEvents = append(recentEvents, ev)
			if len(recentEvents) > recentEventsMax {
				recentEvents = recentEvents[len(recentEvents)-recentEventsMax:]
			}
			recentEventsMu.Unlock()

			log.Printf("evdev: key press on %s — type=%d code=%d value=%d", device, typ, code, value)
		}

		if typ == evKey && code == keyBack && value == 1 {
			evdevBackPressed.Store(true)
		}
	}
}

// EvdevBackJustPressed returns true once if the evdev KEY_BACK was pressed,
// then resets the flag.
func EvdevBackJustPressed() bool {
	return evdevBackPressed.CompareAndSwap(true, false)
}

// EvdevRecentEvents returns a snapshot of the most recent evdev key-press events.
func EvdevRecentEvents() []EvdevEvent {
	recentEventsMu.Lock()
	defer recentEventsMu.Unlock()
	out := make([]EvdevEvent, len(recentEvents))
	copy(out, recentEvents)
	return out
}
