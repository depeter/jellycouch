package player

import (
	"image"
	"os"
)

// PrepareOverlayImage converts an image.Image to raw BGRA pixel data and writes
// it to outPath. Returns the image dimensions. The output file is suitable for
// mpv's overlay-add command.
func PrepareOverlayImage(img image.Image, outPath string) (w, h int, err error) {
	bounds := img.Bounds()
	w = bounds.Dx()
	h = bounds.Dy()

	buf := make([]byte, w*h*4)
	off := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// BGRA byte order, 8-bit per channel
			buf[off] = byte(b >> 8)
			buf[off+1] = byte(g >> 8)
			buf[off+2] = byte(r >> 8)
			buf[off+3] = byte(a >> 8)
			off += 4
		}
	}

	err = os.WriteFile(outPath, buf, 0o644)
	return
}
