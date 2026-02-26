//go:build linux

package ui

import (
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
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

		if typ == evKey && code == keyBack && value == 1 {
			log.Printf("evdev: KEY_BACK detected on %s", path)
			evdevBackPressed.Store(true)
		}
	}
}

// evdevBackJustPressed returns true once if the evdev KEY_BACK was pressed,
// then resets the flag.
func evdevBackJustPressed() bool {
	return evdevBackPressed.CompareAndSwap(true, false)
}
