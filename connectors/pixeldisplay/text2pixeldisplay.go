package pixeldisplay

import (
	"embed"
	"image"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed font/*
var staticContent embed.FS

type text2pixeldisplay struct {
	display Pixeldisplay
}

func NewText2Pixeldisplay(display Pixeldisplay) *text2pixeldisplay {
	return &text2pixeldisplay{display: display}
}

func (t *text2pixeldisplay) DrawText(text string) *base.OperatorIO {
	const (
		width        = 16
		height       = 8
		startingDotX = 1
		startingDotY = 7
	)

	dim := t.display.GetDimensions()
	r := image.Rect(0, 0, dim.X, dim.Y)
	dst := image.NewRGBA(r)

	fontBytes, err := staticContent.ReadFile("font/Grand9K Pixel.ttf")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Reading file from embed fs: %v", err)
	}
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Parse: %v", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    32,
		DPI:     18,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "NewFace: %v", err)
	}

	drawer := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(t.display.GetColor()),
		Face: face,
		Dot:  fixed.P(startingDotX, startingDotY),
	}
	//	if alignRight {
	//		endDot := d.MeasureString(s)
	//		toMove := width - endDot.Ceil()
	//		if toMove > 0 {
	//			d.Dot = fixed.P(startingDotX+toMove, startingDotY)
	//		}
	//	}
	drawer.DrawString(text)
	return t.display.DrawImage(dst)
}
