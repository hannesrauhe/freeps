package pixeldisplay

import (
	"embed"
	"image"
	"image/color"
	"image/draw"
	"net/http"
	"sync"

	"github.com/hannesrauhe/freeps/base"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed font/*
var staticContent embed.FS

type text2pixeldisplay struct {
	display Pixeldisplay
	lock    sync.Mutex
}

func NewText2Pixeldisplay(display Pixeldisplay) *text2pixeldisplay {
	return &text2pixeldisplay{display: display, lock: sync.Mutex{}}
}

func (t *text2pixeldisplay) DrawText(ctx *base.Context, text string) *base.OperatorIO {
	const (
		startingDotX = 1
		startingDotY = 7
	)

	maxDim := t.display.GetMaxPictureSize()
	r := image.Rect(0, 0, maxDim.X, maxDim.Y) // crop the picture later
	dst := image.NewRGBA(r)
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.Transparent), image.Point{}, draw.Src)

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
	dst.Rect.Max.X = drawer.Dot.X.Ceil() // crop the picture

	first := t.display.DrawImage(ctx, dst, true)
	if first.IsError() {
		return first
	}

	return first
}
