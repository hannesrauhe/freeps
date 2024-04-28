package pixeldisplay

import (
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
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
	t2p     *text2pixeldisplay
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
		MinDisplayDuration: 200 * time.Millisecond,
		ImageQueueSize:     5,   // queue can consist of 5 animations...
		MaxAnimationSize:   150, // ... with a maximum of 50 pics each
	},
	}
}

func (op *OpPixelDisplay) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*OpConfig)
	disp, err := NewWLEDMatrixDisplay(cfg.WLEDMatrixDisplay)
	if err != nil {
		return nil, err
	}
	newOp := &OpPixelDisplay{config: *config.(*OpConfig), display: disp, t2p: NewText2Pixeldisplay(disp)}
	return newOp, nil
}

// StartListening is a noop
func (op *OpPixelDisplay) StartListening(ctx *base.Context) {
}

// Shutdown shuts down the display
func (op *OpPixelDisplay) Shutdown(ctx *base.Context) {
	op.display.Shutdown(ctx)
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
	colorStr := utils.GetHexColor(op.display.GetColor())
	return base.MakePlainOutput(colorStr)
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
	if d.IsOn() {
		return base.MakeEmptyOutput()
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "Display is off")
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
	d.SetColor(c)
	return base.MakeEmptyOutput()
}

func (op *OpPixelDisplay) SetBackgroundColor(ctx *base.Context, input *base.OperatorIO, args ColorArgs) *base.OperatorIO {
	d := op.display
	c, err := utils.ParseColor(args.Color)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "color %v not a valid hex color", args.Color)
	}
	d.SetBackgroundColor(c)
	return base.MakeEmptyOutput()
}

type BrightnessArgs struct {
	Brightness int
}

func (op *OpPixelDisplay) SetBrightness(ctx *base.Context, input *base.OperatorIO, args BrightnessArgs) *base.OperatorIO {
	d := op.display
	return d.SetBrightness(args.Brightness)
}

type TextArgs struct {
	Text  *string
	Align *TextAlignment
}

func (op *OpPixelDisplay) DrawText(ctx *base.Context, input *base.OperatorIO, args TextArgs) *base.OperatorIO {
	text := ""
	if !input.IsEmpty() {
		text = input.GetString()
	}
	if args.Text != nil {
		text = *args.Text
	}
	align := Left
	if args.Align != nil {
		align = *args.Align
	}
	return op.t2p.DrawText(ctx, text, align)
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
