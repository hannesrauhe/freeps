package pixeldisplay

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
)

func (op *OpPixelDisplay) getImageFromInput(ctx *base.Context, input *base.OperatorIO) (image.Image, *base.OperatorIO) {
	var binput []byte
	var contentType string
	var img image.Image
	var err error

	if input.IsEmpty() {
		return nil, base.MakeOutputError(http.StatusBadRequest, "no input, expecting an image")
	}

	binput, err = input.GetBytes()
	if err != nil {
		return img, base.MakeOutputError(http.StatusBadRequest, "Could not read input: %v", err.Error())
	}
	contentType = input.ContentType

	ctx.GetLogger().Debugf("Decoding image of type: %v", contentType)
	if contentType == "image/png" {
		img, err = png.Decode(bytes.NewReader(binput))
	} else if contentType == "image/jpeg" {
		img, err = jpeg.Decode(bytes.NewReader(binput))
	} else {
		img, contentType, err = image.Decode(bytes.NewReader(binput))
		ctx.GetLogger().Debugf("Detected type: %v", contentType)
	}
	if err != nil {
		return img, base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return img, base.MakeEmptyOutput()
}

func (op *OpPixelDisplay) getImageFromStore(ctx *base.Context, name *string) (image.Image, *base.OperatorIO) {
	if name == nil {
		return nil, base.MakeEmptyOutput()
	}
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_pixeldisplay")
	if err != nil {
		return nil, base.MakeOutputError(http.StatusInternalServerError, "Cannot access namespace \"%v\": %v", "_pixeldisplay", err.Error())
	}
	iconF := ns.GetValue(*name)
	if iconF.IsError() {
		return nil, iconF.GetData()
	}
	binput, err := iconF.GetData().GetBytes()
	if err != nil {
		return nil, base.MakeOutputError(http.StatusBadRequest, "Icon %v is not accssible: %v", name, err.Error())
	}
	img, contentType, err := image.Decode(bytes.NewReader(binput))
	ctx.GetLogger().Debugf("Detected type when loading from store: %v", contentType)
	return img, base.MakeEmptyOutput()
}

func (op *OpPixelDisplay) getDrawablePicture(src image.Image) *image.RGBA {
	if src == nil {
		src = image.NewUniform(op.display.GetBackgroundColor())
	}
	dim := op.display.GetDimensions()
	r := image.Rect(0, 0, dim.X, dim.Y)
	dst := image.NewRGBA(r)
	draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
	return dst
}

type ImageNameArgs struct {
	ImageName *string
}

func (op *OpPixelDisplay) ImageNameSuggestions() []string {
	ns, _ := freepsstore.GetGlobalStore().GetNamespace("_pixeldisplay")
	if ns == nil {
		return []string{}
	}
	return ns.GetKeys()
}

// GetPixelMatrix returns the string representation of an image (used in the UI)
func (op *OpPixelDisplay) GetPixelMatrix(ctx *base.Context, input *base.OperatorIO, args ImageNameArgs) *base.OperatorIO {
	var pic *image.RGBA
	if args.ImageName != nil {
		img, _ := op.getImageFromStore(ctx, args.ImageName)
		pic = op.getDrawablePicture(img) // gives back an empty canvase on error
	} else if !input.IsEmpty() {
		img, out := op.getImageFromInput(ctx, input)
		if out.IsError() {
			return out
		}
		pic = op.getDrawablePicture(img)
	} else {
		pic = op.getDrawablePicture(nil)
	}

	pm := make([][]string, 0)
	for y := 0; y < pic.Bounds().Dy(); y++ {
		pm = append(pm, make([]string, pic.Bounds().Dx()))
		for x := 0; x < pic.Bounds().Dx(); x++ {
			c := pic.At(x, y)
			pm[y][x] = utils.GetHexColor(c)
		}
	}
	return base.MakeObjectOutput(pm)
}

func (op *OpPixelDisplay) DrawImage(ctx *base.Context, input *base.OperatorIO, args ImageNameArgs) *base.OperatorIO {
	var img image.Image
	var out *base.OperatorIO
	if args.ImageName != nil {
		img, out = op.getImageFromStore(ctx, args.ImageName)
		if out.IsError() {
			return out
		}
	} else if !input.IsEmpty() {
		img, out = op.getImageFromInput(ctx, input)
		if out.IsError() {
			return out
		}
	} else {
		return base.MakeOutputError(http.StatusBadRequest, "No image provided")
	}
	return op.display.DrawImage(ctx, img, true)
}

type SetBackgroundLayerArgs struct {
	LayerName *string
	ImageName *string
}

func (op *OpPixelDisplay) SetBackgroundLayer(ctx *base.Context, input *base.OperatorIO, args SetBackgroundLayerArgs) *base.OperatorIO {
	var img image.Image
	var out *base.OperatorIO
	if args.ImageName != nil {
		img, out = op.getImageFromStore(ctx, args.ImageName)
		if out.IsError() {
			return out
		}
	} else if !input.IsEmpty() {
		img, out = op.getImageFromInput(ctx, input)
		if out.IsError() {
			return out
		}
	} else {
		return base.MakeOutputError(http.StatusBadRequest, "No image provided")
	}
	layer := "default"
	if args.LayerName != nil {
		layer = *args.LayerName
	}
	return op.display.SetBackgroundLayer(ctx, img, layer)
}

type ResetBackgroundLayerArgs struct {
	LayerName string
}

func (op *OpPixelDisplay) ResetBackgroundLayer(ctx *base.Context, input *base.OperatorIO, args ResetBackgroundLayerArgs) *base.OperatorIO {
	return op.display.SetBackgroundLayer(ctx, nil, args.LayerName)
}

func (op *OpPixelDisplay) ResetBackground(ctx *base.Context) *base.OperatorIO {
	return op.display.ResetBackground(ctx)
}

type DrawPixelArg struct {
	X         int
	Y         int
	ImageName string
	Color     *string
}

// DrawPixel puts a pixel on the image stored under ImageName and displays the Image
func (op *OpPixelDisplay) DrawPixel(ctx *base.Context, input *base.OperatorIO, args DrawPixelArg) *base.OperatorIO {
	img, _ := op.getImageFromStore(ctx, &args.ImageName)
	pic := op.getDrawablePicture(img) // gives back an empty canvase on error

	c := op.display.GetColor()
	if args.Color != nil {
		var err error
		c, err = utils.ParseColor(*args.Color)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Cannot draw pixel with color \"%v\":%v", *args.Color, err.Error())
		}
	}
	pic.Set(args.X, args.Y, c)

	out := op.display.DrawImage(ctx, pic, true)
	if out.IsError() {
		return out
	}

	ns, err := freepsstore.GetGlobalStore().GetNamespace("_pixeldisplay")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot access namespace \"%v\": %v", "_pixeldisplay", err.Error())
	}
	e := ns.SetValue(args.ImageName, out, ctx)
	if e.IsError() {
		return e.GetData()
	}
	return base.MakeEmptyOutput()
}
