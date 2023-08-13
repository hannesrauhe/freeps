package pixeldisplay

import (
	"bytes"
	"fmt"
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
	Enabled            bool                               `json:"enabled"`
	WLEDMatrixDisplays map[string]WLEDMatrixDisplayConfig `json:"wledMatrixDisplays"`
	DefaultDisplay     string                             `json:"defaultDisplay"`
}

var defaultConfig = OpConfig{DefaultDisplay: "default", WLEDMatrixDisplays: map[string]WLEDMatrixDisplayConfig{
	"default": {
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
}}

// OpPixelDisplay implements base.FreepsOperatorWithShutdown, wraps all functions of the Pixeldisplay interface and calls them on the default display
type OpPixelDisplay struct {
	config   *OpConfig
	displays map[string]Pixeldisplay
}

func (op *OpPixelDisplay) ResetConfigToDefault() interface{} {
	op.config = &defaultConfig
	return op.config
}

func (op *OpPixelDisplay) Init(ctx *base.Context) error {
	op.displays = make(map[string]Pixeldisplay)
	for name, cfg := range op.config.WLEDMatrixDisplays {
		// check if the display is already initialized
		if _, ok := op.displays[name]; ok {
			return fmt.Errorf("display %s already exists", name)
		}
		disp, err := NewWLEDMatrixDisplay(cfg)
		if err != nil {
			return err
		}
		op.displays[name] = disp
	}
	return nil
}

func (op *OpPixelDisplay) Shutdown(ctx *base.Context) {
	for _, disp := range op.displays {
		disp.Shutdown()
	}
}

type PixeldisplayArgs struct {
	Display *string
}

var _ base.FreepsFunctionParameters = &PixeldisplayArgs{}

// GetArgSuggestions returns a map of possible arguments for the given function and argument name
func (mf *PixeldisplayArgs) GetArgSuggestions(opBase base.FreepsOperator, fn string, argName string, otherArgs map[string]string) map[string]string {
	op := opBase.(*OpPixelDisplay)
	res := make(map[string]string)
	switch argName {
	case "display":
		for name := range op.displays {
			res[name] = name
		}
		return res
	default:
		return nil
	}
}

// InitOptionalParameters initializes the optional (pointer) arguments of the parameters struct with default values
func (mf *PixeldisplayArgs) InitOptionalParameters(op base.FreepsOperator, fn string) {
	mf.Display = new(string)
	*mf.Display = op.(*OpPixelDisplay).config.DefaultDisplay
}

// VerifyParameters checks if the given parameters are valid
func (mf *PixeldisplayArgs) VerifyParameters(op base.FreepsOperator) *base.OperatorIO {
	if mf.Display == nil {
		return base.MakeOutputError(http.StatusBadRequest, "Missing display")
	}
	if _, ok := op.(*OpPixelDisplay).displays[*mf.Display]; !ok {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid display")
	}
	return nil
}

// GetDisplay returns the display given by the arguments
func (op *OpPixelDisplay) GetDisplay(args PixeldisplayArgs) Pixeldisplay {
	return op.displays[*args.Display]
}

func (op *OpPixelDisplay) TurnOn(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return d.TurnOn()
}

func (op *OpPixelDisplay) TurnOff(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return d.TurnOff()
}

func (op *OpPixelDisplay) GetDimensions(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return base.MakeObjectOutput(d.GetDimensions())
}

func (op *OpPixelDisplay) GetMaxPictureSize(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return base.MakeObjectOutput(d.GetMaxPictureSize())
}

func (op *OpPixelDisplay) GetColor(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return base.MakeObjectOutput(d.GetColor())
}

func (op *OpPixelDisplay) GetBackgroundColor(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return base.MakeObjectOutput(d.GetBackgroundColor())
}

func (op *OpPixelDisplay) GetBrightness(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return base.MakeObjectOutput(d.GetBrightness())
}

func (op *OpPixelDisplay) IsOn(ctx *base.Context, input *base.OperatorIO, args PixeldisplayArgs) *base.OperatorIO {
	d := op.GetDisplay(args)
	return base.MakeObjectOutput(d.IsOn())
}

type ColorArgs struct {
	PixeldisplayArgs
	Color string
}

func (op *OpPixelDisplay) SetColor(ctx *base.Context, input *base.OperatorIO, args ColorArgs) *base.OperatorIO {
	d := op.GetDisplay(args.PixeldisplayArgs)
	c, err := utils.ParseHexColor(args.Color)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "color %v not a valid hex color", args.Color)
	}
	return d.SetColor(c)
}

func (op *OpPixelDisplay) SetBackgroundColor(ctx *base.Context, input *base.OperatorIO, args ColorArgs) *base.OperatorIO {
	d := op.GetDisplay(args.PixeldisplayArgs)
	c, err := utils.ParseHexColor(args.Color)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "color %v not a valid hex color", args.Color)
	}
	return d.SetBackgroundColor(c)
}

type BrightnessArgs struct {
	PixeldisplayArgs
	Brightness int
}

func (op *OpPixelDisplay) SetBrightness(ctx *base.Context, input *base.OperatorIO, args BrightnessArgs) *base.OperatorIO {
	d := op.GetDisplay(args.PixeldisplayArgs)
	return d.SetBrightness(args.Brightness)
}

type TextArgs struct {
	PixeldisplayArgs
	Text string
}

func (op *OpPixelDisplay) DrawText(ctx *base.Context, input *base.OperatorIO, args TextArgs) *base.OperatorIO {
	d := op.GetDisplay(args.PixeldisplayArgs)
	t := NewText2Pixeldisplay(d)
	return t.DrawText(args.Text)
}

type ImageArgs struct {
	PixeldisplayArgs
	Icon *string
}

func (op *OpPixelDisplay) DrawImage(ctx *base.Context, input *base.OperatorIO, args ImageArgs) *base.OperatorIO {
	d := op.GetDisplay(args.PixeldisplayArgs)
	var binput []byte
	var contentType string
	var img image.Image
	var err error

	if args.Icon != nil {
		binput, err = freepsstore.GetFileStore().GetValue(*args.Icon).GetBytes()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Icon %v is not accssible: %v", err.Error())
		}
		contentType = "image/png"
	} else if input.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no input, expecting an image")
	} else {
		binput, err = input.GetBytes()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Could not read input: %v", err.Error())
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
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	// dim := d.GetDimensions()
	// r := image.Rect(0, 0, dim.X, dim.Y)
	// dst := image.NewRGBA(r)
	// draw.NearestNeighbor.Scale(dst, r, img, img.Bounds(), draw.Over, nil)
	return d.DrawImage(img, true)
}
