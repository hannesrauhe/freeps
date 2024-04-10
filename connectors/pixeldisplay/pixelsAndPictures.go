package pixeldisplay

import (
	"image"
	"image/draw"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

func (op *OpPixelDisplay) resetLast(src image.Image) {
	if src == nil {
		src = image.NewUniform(op.display.GetBackgroundColor())
	}
	dim := op.display.GetDimensions()
	r := image.Rect(0, 0, dim.X, dim.Y)
	dst := image.NewRGBA(r)
	draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
	op.last = dst
}

// GetPixelMatrix returns the string representation of an image (used in the UI)
func (op *OpPixelDisplay) GetPixelMatrix(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	if input.IsEmpty() {
		if op.last == nil {
			op.resetLast(nil)
		}
	} else {
		img, out := op.getImageFromInput(ctx, input, nil)
		if out.IsError() {
			return out
		}
		op.resetLast(img)
	}

	pm := make([][]string, 0)
	for y := 0; y < op.last.Bounds().Dy(); y++ {
		pm = append(pm, make([]string, op.last.Bounds().Dx()))
		for x := 0; x < op.last.Bounds().Dx(); x++ {
			c := op.last.At(x, y)
			pm[y][x] = utils.GetHexColor(c)
		}
	}
	return base.MakeObjectOutput(pm)
}

type DrawPixelArg struct {
	X     int
	Y     int
	Color *string
}

func (op *OpPixelDisplay) DrawPixel(ctx *base.Context, input *base.OperatorIO, args DrawPixelArg) *base.OperatorIO {
	if op.last == nil {
		op.resetLast(nil)
	}

	c := op.display.GetColor()
	op.last.Set(args.X, args.Y, c)

	return op.display.DrawImage(ctx, op.last, false)
}

func (op *OpPixelDisplay) SetLastAsBackground(ctx *base.Context) *base.OperatorIO {
	d := op.display

	return d.SetBackgroundImage(ctx, op.last)
}
func (op *OpPixelDisplay) ResetBackground(ctx *base.Context) *base.OperatorIO {
	d := op.display

	return d.SetBackgroundImage(ctx, nil)
}
