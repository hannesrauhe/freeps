package pixeldisplay

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
)

// OpConfig contains all parameters to initialize the available displays
type OpConfig struct {
	Enabled           bool                    `json:"enabled"`
	WLEDMatrixDisplay WLEDMatrixDisplayConfig `json:"wledMatrixDisplay"`
}

// OpPixelDisplay implements base.FreepsOperatorWithShutdown, wraps all functions of the Pixeldisplay interface and calls them on the default display
type OpPixelDisplay struct {
	config  OpConfig
	display Pixeldisplay
	last    *image.RGBA
}

var _ base.FreepsOperatorWithShutdown = &OpPixelDisplay{}

func (op *OpPixelDisplay) GetDefaultConfig() interface{} {
	return &OpConfig{Enabled: false, WLEDMatrixDisplay: WLEDMatrixDisplayConfig{
		Address: "http://10.0.0.1",
		Segments: []WLEDSegmentConfig{
			{
				Width:  16,
				Height: 16,
				SegID:  0,
			},
		},
		MinDisplayDuration:    200 * time.Millisecond,
		MaxPictureWidthFactor: 50,
	},
	}
}

func (op *OpPixelDisplay) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*OpConfig)
	disp, err := NewWLEDMatrixDisplay(cfg.WLEDMatrixDisplay)
	if err != nil {
		return nil, err
	}
	newOp := &OpPixelDisplay{config: *config.(*OpConfig), display: disp}
	return newOp, nil
}

// StartListening is a noop
func (op *OpPixelDisplay) StartListening(ctx *base.Context) {
}

// Shutdown shuts down the display
func (op *OpPixelDisplay) Shutdown(ctx *base.Context) {
	op.display.Shutdown()
}

func (op *OpPixelDisplay) TurnOn(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return d.TurnOn()
}

func (op *OpPixelDisplay) TurnOff(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return d.TurnOff()
}

func (op *OpPixelDisplay) GetDimensions(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return base.MakeObjectOutput(d.GetDimensions())
}

func (op *OpPixelDisplay) GetMaxPictureSize(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return base.MakeObjectOutput(d.GetMaxPictureSize())
}

func (op *OpPixelDisplay) GetColor(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return base.MakeObjectOutput(utils.GetHexColor(d.GetColor()))
}

func (op *OpPixelDisplay) GetBackgroundColor(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return base.MakeObjectOutput(utils.GetHexColor(d.GetBackgroundColor()))
}

func (op *OpPixelDisplay) GetBrightness(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return base.MakeObjectOutput(d.GetBrightness())
}

func (op *OpPixelDisplay) IsOn(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	d := op.display
	return base.MakeObjectOutput(d.IsOn())
}

type ColorArgs struct {
	Color string
}

func (op *OpPixelDisplay) SetColor(ctx *base.Context, input *base.OperatorIO, args ColorArgs) *base.OperatorIO {
	d := op.display
	c, err := utils.ParseColor(args.Color)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "color %v not a valid hex color", args.Color)
	}
	out := d.SetColor(c)
	if out.IsError() {
		return out
	}
	return base.MakeEmptyOutput()
}

func (op *OpPixelDisplay) SetBackgroundColor(ctx *base.Context, input *base.OperatorIO, args ColorArgs) *base.OperatorIO {
	d := op.display
	c, err := utils.ParseColor(args.Color)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "color %v not a valid hex color", args.Color)
	}
	return d.SetBackgroundColor(c)
}

type BrightnessArgs struct {
	Brightness int
}

func (op *OpPixelDisplay) SetBrightness(ctx *base.Context, input *base.OperatorIO, args BrightnessArgs) *base.OperatorIO {
	d := op.display
	return d.SetBrightness(args.Brightness)
}

type TextArgs struct {
	Text *string
}

func (op *OpPixelDisplay) DrawText(ctx *base.Context, input *base.OperatorIO, args TextArgs) *base.OperatorIO {
	d := op.display
	t := NewText2Pixeldisplay(d)
	text := ""
	if !input.IsEmpty() {
		text = input.GetString()
	}
	if args.Text != nil {
		text = *args.Text
	}
	return t.DrawText(ctx, text)
}

type ImageArgs struct {
	Icon *string
}

func (op *OpPixelDisplay) getImageFromInput(ctx *base.Context, input *base.OperatorIO, icon *string) (image.Image, *base.OperatorIO) {
	var binput []byte
	var contentType string
	var img image.Image
	var err error

	if icon != nil {
		iconF := freepsstore.GetFileStore().GetValue(*icon)
		if iconF.IsError() {
			return img, iconF.GetData()
		}
		binput, err = iconF.GetData().GetBytes()
		if err != nil {
			return img, base.MakeOutputError(http.StatusBadRequest, "Icon %v is not accssible: %v", *icon, err.Error())
		}
		contentType = "image/png"
	} else if input.IsEmpty() {
		return img, base.MakeOutputError(http.StatusBadRequest, "no input, expecting an image")
	} else {
		binput, err = input.GetBytes()
		if err != nil {
			return img, base.MakeOutputError(http.StatusBadRequest, "Could not read input: %v", err.Error())
		}
		contentType = input.ContentType
	}

	ctx.GetLogger().Debugf("Decoding image of type: %v", contentType)
	if contentType == "image/png" {
		img, err = png.Decode(bytes.NewReader(binput))
	} else if contentType == "image/jpeg" {
		img, err = jpeg.Decode(bytes.NewReader(binput))
	} else {
		img, _, err = image.Decode(bytes.NewReader(binput))
	}
	if err != nil {
		return img, base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return img, base.MakeObjectOutput(img)
}

func (op *OpPixelDisplay) DrawImage(ctx *base.Context, input *base.OperatorIO, args ImageArgs) *base.OperatorIO {
	d := op.display

	img, out := op.getImageFromInput(ctx, input, args.Icon)
	if out.IsError() {
		return out
	}
	return d.DrawImage(ctx, img, true)
}

// EffectArgs is a struct to hold the effect to set
type EffectArgs struct {
	Fx int
}

// SetEffect sets the effect
func (op *OpPixelDisplay) SetEffect(ctx *base.Context, input *base.OperatorIO, args EffectArgs) *base.OperatorIO {
	d := op.display
	return d.SetEffect(args.Fx)
}
