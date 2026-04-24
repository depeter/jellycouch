// gen-ico writes a multi-resolution Windows .ico file using the same icon
// artwork the app uses at runtime. Run from the repo root:
//
//	go run ./cmd/gen-ico installer/jellycouch.ico
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"io"
	"os"

	"github.com/depeter/jellycouch/assets/icon"
)

// Sizes shipped inside the .ico. Windows picks the closest match at display time.
var sizes = []int{16, 32, 48, 64, 128, 256}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen-ico <output.ico>")
		os.Exit(2)
	}
	out := os.Args[1]

	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", out, err)
		os.Exit(1)
	}
	defer f.Close()

	if err := writeICO(f, sizes); err != nil {
		fmt.Fprintf(os.Stderr, "write ico: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d sizes)\n", out, len(sizes))
}

// writeICO emits an .ico with PNG-encoded entries (valid on Vista+).
// Format: ICONDIR header + N ICONDIRENTRY records + concatenated PNG payloads.
func writeICO(w io.Writer, sizes []int) error {
	pngs := make([][]byte, len(sizes))
	for i, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, icon.GenerateSize(s)); err != nil {
			return fmt.Errorf("encode png %d: %w", s, err)
		}
		pngs[i] = buf.Bytes()
	}

	// ICONDIR: reserved(2)=0, type(2)=1 (icon), count(2)=N
	if err := binary.Write(w, binary.LittleEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(sizes))); err != nil {
		return err
	}

	// Offset of the first payload = 6 (ICONDIR) + 16 * N (ICONDIRENTRY).
	offset := uint32(6 + 16*len(sizes))
	for i, s := range sizes {
		// Width/height are stored as a single byte. 0 represents 256.
		wh := byte(s)
		if s >= 256 {
			wh = 0
		}
		entry := struct {
			Width, Height, Colors, Reserved byte
			Planes, BitCount                uint16
			Size, Offset                    uint32
		}{
			Width:    wh,
			Height:   wh,
			Colors:   0,
			Reserved: 0,
			Planes:   1,
			BitCount: 32,
			Size:     uint32(len(pngs[i])),
			Offset:   offset,
		}
		if err := binary.Write(w, binary.LittleEndian, entry); err != nil {
			return err
		}
		offset += uint32(len(pngs[i]))
	}

	for _, p := range pngs {
		if _, err := w.Write(p); err != nil {
			return err
		}
	}
	return nil
}
