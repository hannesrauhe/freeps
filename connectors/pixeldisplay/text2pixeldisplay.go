package pixeldisplay

import (
	"embed"
	"image"
	"image/color"
	"image/draw"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed font/*
var staticContent embed.FS

type TextAlignment string

const (
	Left   TextAlignment = "left"
	Center               = "center"
	Right                = "right"
)

type text2pixeldisplay struct {
	display Pixeldisplay
}

func NewText2Pixeldisplay(display Pixeldisplay) *text2pixeldisplay {
	return &text2pixeldisplay{display: display}
}

func (t *text2pixeldisplay) DrawText(ctx *base.Context, text string, align TextAlignment) *base.OperatorIO {
	const (
		startingDotX = 1
		startingDotY = 7
	)

	maxDim := t.display.GetMaxPictureSize()
	r := image.Rect(0, 0, maxDim.X, maxDim.Y)
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
	if align != Left {
		dim := t.display.GetDimensions()
		endDot := drawer.MeasureString(text)
		endX := dim.X - endDot.Ceil()
		if endX > 0 {
			if align == Right {
				drawer.Dot = fixed.P(startingDotX+endX, startingDotY)
			} else if align == Center {
				drawer.Dot = fixed.P(startingDotX+endX/2, startingDotY)
			}
		}
	}

	drawer.DrawString(text)
	if drawer.Dot.X.Ceil() < maxDim.X {
		// crop the generated picture - will not modify the picture itself, just set the bounds
		dst.Rect.Max.X = drawer.Dot.X.Ceil()
	}

	first := t.display.DrawImage(ctx, dst, true)
	if first.IsError() {
		return first
	}

	return first
}
