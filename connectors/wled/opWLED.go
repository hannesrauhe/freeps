package wled

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
)

// OpWLED is the operator for the WLED connector
type OpWLED struct {
	config       OpConfig
	defaultColor color.Color
}

var _ base.FreepsOperatorWithConfig = &OpWLED{}

// GetConfig returns the default config for the WLED connector
func (o *OpWLED) GetConfig() interface{} {
	o.config = DefaultConfig
	return &o.config
}

// Init initializes the WLED connector
func (o *OpWLED) Init(ctx *base.Context) error {
	o.defaultColor = image.White
	activeConnection := o.config.DefaultConnection
	w, err := NewWLEDConverter(activeConnection, o.config.Connections)
	if err != nil {
		return err
	}
	return w.PrepareStore()
}

// CommonArgs are common arguments for all WLED functions
type CommonArgs struct {
	Config      *string
	PixelMatrix *string
	ShowImage   *bool

	activeConnection string
	w                *WLEDConverter
}

func (args *CommonArgs) setUpConnection(config *OpConfig) *base.OperatorIO {
	args.activeConnection = config.DefaultConnection
	if args.Config != nil {
		args.activeConnection = *args.Config
	}
	var err error
	args.w, err = NewWLEDConverter(args.activeConnection, config.Connections)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Creating Connection to WLED failed: %v", err.Error())
	}
	return nil
}

func (args *CommonArgs) finish(ctx *base.Context) *base.OperatorIO {
	showImage := false
	if args.ShowImage != nil {
		showImage = *args.ShowImage
	}
	ret := args.w.SendToWLED(nil, showImage)
	if args.PixelMatrix != nil {
		args.w.StorePixelMatrix(ctx, *args.PixelMatrix)
	}
	args.w.StorePixelMatrix(ctx, args.activeConnection)
	args.w.StorePixelMatrix(ctx, "last")
	return ret
}

// CmdArgs are arguments for the sendCmd function
type CmdArgs struct {
	CommonArgs
	Cmd string
}

// SendCMD sends a command to WLED
func (o *OpWLED) SendCMD(ctx *base.Context, mainInput *base.OperatorIO, args CmdArgs) *base.OperatorIO {
	err := args.setUpConnection(&o.config)
	if err != nil {
		return err
	}

	switch args.Cmd {
	case "on":
		return args.w.SendToWLED(base.MakeObjectOutput(&WLEDState{On: true}), false)
	case "off":
		return args.w.SendToWLED(base.MakeObjectOutput(&WLEDState{On: false}), false)
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown command: %v", args.Cmd)
}

// SetImageArgs are arguments for the setImage function
type SetImageArgs struct {
	CommonArgs
	Icon *string
}

// SetImage sets an image on the WLED
func (o *OpWLED) SetImage(ctx *base.Context, mainInput *base.OperatorIO, args SetImageArgs) *base.OperatorIO {
	setupErr := args.setUpConnection(&o.config)
	if setupErr != nil {
		return setupErr
	}

	var binput []byte
	var contentType string
	var img image.Image
	var err error

	if args.Icon != nil {
		binput, err = staticContent.ReadFile("font/" + *args.Icon + ".png")
		contentType = "image/png"
	} else if mainInput.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no input, expecting an image")
	} else {
		binput, err = mainInput.GetBytes()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
		contentType = mainInput.ContentType
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
	args.w.ScaleImage(img)
	return args.finish(ctx)
}

// SetStringArgs are arguments for the setString function
type SetStringArgs struct {
	CommonArgs
	String     *string
	AlignRight *bool
	Color      *string
}

// SetString sets a string on the WLED
func (o *OpWLED) SetString(ctx *base.Context, mainInput *base.OperatorIO, args SetStringArgs) *base.OperatorIO {
	setUpErr := args.setUpConnection(&o.config)
	if setUpErr != nil {
		return setUpErr
	}
	var err error
	c := o.defaultColor
	str := mainInput.GetString()
	if args.String != nil {
		str = *args.String
	}
	if args.Color != nil {
		c, err = utils.ParseHexColor(*args.Color)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "color \"%s\" not a valid hex color", *args.Color)
		}
	}
	alignRight := false
	if args.AlignRight != nil {
		alignRight = *args.AlignRight
	}
	err = args.w.WriteString(str, c, alignRight)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return args.finish(ctx)
}

// DrawPixelArgs are arguments for the drawPixel function
type DrawPixelArgs struct {
	CommonArgs
	X     int
	Y     int
	Color *string
}

// DrawPixel draws a pixel on the WLED
func (o *OpWLED) DrawPixel(ctx *base.Context, mainInput *base.OperatorIO, args DrawPixelArgs) *base.OperatorIO {
	setUpErr := args.setUpConnection(&o.config)
	if setUpErr != nil {
		return setUpErr
	}
	var err error
	c := o.defaultColor
	if args.Color != nil {
		c, err = utils.ParseHexColor(*args.Color)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "color \"%s\" not a valid hex color", *args.Color)
		}
	}
	err = args.w.SetPixel(args.X, args.Y, c)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return args.finish(ctx)
}

// DrawPixelMatrixArgs are arguments for the drawPixelMatrix function
type DrawPixelMatrixArgs struct {
	CommonArgs
	AnimationType        *string
	StepDurationInMillis *int `json:",string"`
	Repeat               *int `json:",string"`
}

var _ base.FreepsFunctionParameters = &DrawPixelMatrixArgs{}

// InitOptionalParameters initializes the optional parameters
func (args *DrawPixelMatrixArgs) InitOptionalParameters(fn string) {
	args.Repeat = new(int)
	*args.Repeat = 0
	args.StepDurationInMillis = new(int)
	*args.StepDurationInMillis = 500
	args.AnimationType = new(string)
	*args.AnimationType = "static"
}

// GetArgSuggestions returns suggestions for the color
func (args *DrawPixelMatrixArgs) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "animationtype":
		return map[string]string{"move": "move", "shift": "shift", "moveLeft": "moveLeft", "shiftLeft": "shiftLeft", "squence": "sequence", "static": "static"}
	case "showimage", "alignright":
		return map[string]string{"true": "true", "false": "false"}
	case "pixelmatrix":
		m := map[string]string{}
		wledNs := freepsstore.GetGlobalStore().GetNamespace("_wled")
		for _, k := range wledNs.GetKeys() {
			m[k] = k
		}
		return m
	case "cmd":
		return map[string]string{"on": "on", "off": "off"}
	}
	return map[string]string{}
}

// DrawPixelMatrix draws a pixelmatrix on the WLED
func (o *OpWLED) DrawPixelMatrix(ctx *base.Context, mainInput *base.OperatorIO, animate DrawPixelMatrixArgs) *base.OperatorIO {
	setupErr := animate.setUpConnection(&o.config)
	if setupErr != nil {
		return setupErr
	}
	animate.w.SetPixelMatrix(pmName) // if error, this will just draw an empty one
	pm := animate.w.GetPixelMatrix()

	for r := 0; r <= *animate.Repeat; r++ {
		switch *animate.AnimationType {
		case "move", "moveLeft":
			for i := -1 * len(pm[0]); i < len(pm[0]); i++ {
				if animate.AnimationType == "moveLeft" {
					w.DrawPixelMatrix(pm.MoveLeft("#000000", i))
				} else {
					w.DrawPixelMatrix(pm.MoveRight("#000000", i))
				}
				w.SendToWLED(nil, false)
				time.Sleep(time.Duration(animate.StepDurationInMillis) * time.Millisecond)
			}
		case "shift":
			for i := 0; i < len(pm[0]); i++ {
				w.DrawPixelMatrix(pm.Shift(i))
				w.SendToWLED(nil, false)
				time.Sleep(time.Duration(animate.StepDurationInMillis) * time.Millisecond)
			}
		case "sequence":
			seqName := pmName
			for i := 1; true; i++ {
				err := w.SetPixelMatrix(seqName)
				if err != nil {
					break
				}
				w.SendToWLED(nil, false)
				time.Sleep(time.Duration(animate.StepDurationInMillis) * time.Millisecond)
				seqName = fmt.Sprintf("%v.%d", pmName, i)
			}
		default:
			w.DrawPixelMatrix(pm)
			w.SendToWLED(nil, false)
			return base.MakeEmptyOutput()
		}
	}
	return base.MakeEmptyOutput()
}

func (o *OpWLED) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "animationType":
		return map[string]string{"move": "move", "shift": "shift", "moveLeft": "moveLeft", "shiftLeft": "shiftLeft", "squence": "sequence"}
	case "showImage", "alignRight":
		return map[string]string{"true": "true", "false": "false"}
	case "pixelMatrix":
		m := map[string]string{}
		wledNs := freepsstore.GetGlobalStore().GetNamespace("_wled")
		for _, k := range wledNs.GetKeys() {
			m[k] = k
		}
		return m
	case "config":
		m := map[string]string{}
		for k, _ := range o.config.Connections {
			m[k] = k
		}
		return m
	case "cmd":
		return map[string]string{"on": "on", "off": "off"}
	}
	return map[string]string{}
}

func (o *OpWLED) SetPixelMatrix(w *WLEDConverter, pmName string, animate DrawPixelMatrixArgs) *base.OperatorIO {
}
