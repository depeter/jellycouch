package ui

import (
	"bytes"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	fontSource *text.GoTextFaceSource
	fontFaces  map[float64]*text.GoTextFace
)

func InitFonts(ttfData []byte) error {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(ttfData))
	if err != nil {
		return err
	}
	fontSource = src
	fontFaces = make(map[float64]*text.GoTextFace)
	return nil
}

func GetFace(size float64) *text.GoTextFace {
	if face, ok := fontFaces[size]; ok {
		return face
	}
	face := &text.GoTextFace{
		Source: fontSource,
		Size:   size,
	}
	fontFaces[size] = face
	return face
}

func DrawText(dst *ebiten.Image, txt string, x, y float64, size float64, clr color.Color) {
	face := GetFace(size)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	text.Draw(dst, txt, face, op)
}

func DrawTextCentered(dst *ebiten.Image, txt string, cx, cy float64, size float64, clr color.Color) {
	face := GetFace(size)
	w, h := text.Measure(txt, face, 0)
	DrawText(dst, txt, cx-w/2, cy-h/2, size, clr)
}

func MeasureText(txt string, size float64) (float64, float64) {
	face := GetFace(size)
	return text.Measure(txt, face, 0)
}

func DrawTextWrapped(dst *ebiten.Image, txt string, x, y, maxWidth float64, size float64, clr color.Color) float64 {
	face := GetFace(size)
	lineHeight := face.Size * 1.4
	words := strings.Fields(txt)
	if len(words) == 0 {
		return 0
	}

	line := words[0]
	cy := y
	for _, word := range words[1:] {
		test := line + " " + word
		w, _ := text.Measure(test, face, 0)
		if w > maxWidth {
			DrawText(dst, line, x, cy, size, clr)
			cy += lineHeight
			line = word
		} else {
			line = test
		}
	}
	DrawText(dst, line, x, cy, size, clr)
	cy += lineHeight
	return cy - y
}
