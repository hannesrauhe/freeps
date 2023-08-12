package pixeldisplay

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
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
