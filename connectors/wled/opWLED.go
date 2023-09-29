package wled

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

type OpWLED struct {
	cr     *utils.ConfigReader
	config *OpConfig
}

var _ base.FreepsBaseOperator = &OpWLED{}

// GetName returns the name of the operator
func (o *OpWLED) GetName() string {
	return "wled"
}

func (o *OpWLED) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	activeConnection := o.config.DefaultConnection
	if vars["config"] != "" {
		activeConnection = vars["config"]
	}
	w, err := NewWLEDConverter(activeConnection, o.config.Connections)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid parameters: %v", err.Error())
	}
	pmName := vars["pixelMatrix"]
	if pmName == "" {
		pmName = "last"
	}

	switch function {
	case "sendCmd":
		switch vars["cmd"] {
		case "on":
			return w.SendToWLED(base.MakeObjectOutput(&WLEDState{On: true}), false)
		case "off":
			return w.SendToWLED(base.MakeObjectOutput(&WLEDState{On: false}), false)
		}
		return w.SendToWLED(mainInput, false)
	case "setImage":
		var binput []byte
		var contentType string
		var img image.Image

		if vars["icon"] != "" {
			binput, err = staticContent.ReadFile("font/" + vars["icon"] + ".png")
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
		w.ScaleImage(img)
	case "setString":
		c := image.White.C
		str, ok := vars["string"]
		if !ok {
			str = mainInput.GetString()
		}
		if colstr, ok := vars["color"]; ok {
			c, err = utils.ParseHexColor(colstr)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "color not a valid hex color")
			}
		}
		err = w.WriteString(str, c, utils.ParseBool(vars["alignRight"]))
	case "drawPixel":
		c := image.White.C
		w.SetPixelMatrix(pmName)
		if colstr, ok := vars["color"]; ok {
			c, err = utils.ParseHexColor(colstr)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "color not a valid hex color")
			}
		}
		x, err := strconv.Atoi(vars["x"])
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "x not a valid integer")
		}
		y, err := strconv.Atoi(vars["y"])
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "y not a valid integer")
		}
		err = w.SetPixel(x, y, c)
	case "drawPixelMatrix":
		animate := AnimationOptions{StepDurationInMillis: 500}
		err := utils.ArgsMapToObject(vars, &animate)
		if err != nil {
			return base.MakeOutputError(http.StatusNotFound, "Could not parse Animation Parameters: %v", err)
		}
		return o.SetPixelMatrix(w, pmName, animate)
	default:
		return base.MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	ret := w.SendToWLED(nil, utils.ParseBool(vars["showImage"]))
	w.StorePixelMatrix(ctx, pmName)
	w.StorePixelMatrix(ctx, activeConnection)
	return ret
}

func (o *OpWLED) GetFunctions() []string {
	return []string{"setString", "setImage", "drawPixel", "drawPixelMatrix", "sendCmd"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address", "string", "x", "y", "segid", "icon", "color", "bgcolor", "alignRight", "showImage", "pixelMatrix", "height", "width", "animationType", "cmd", "config"}
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

type AnimationOptions struct {
	AnimationType        string
	StepDurationInMillis int `json:",string"`
	Repeat               int `json:",string"`
}

func (o *OpWLED) SetPixelMatrix(w *WLEDConverter, pmName string, animate AnimationOptions) *base.OperatorIO {
	w.SetPixelMatrix(pmName) // ignore error, this will just draw an empty one
	pm := w.GetPixelMatrix()

	for r := 0; r <= animate.Repeat; r++ {
		switch animate.AnimationType {
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

func NewWLEDOp(cr *utils.ConfigReader) *OpWLED {
	conf := DefaultConfig
	err := cr.ReadSectionWithDefaults("wled", &conf)
	if err != nil {
		logrus.Errorf("Reading wled config failed: %v", err)
	} else {
		err = cr.WriteBackConfigIfChanged()
		if err != nil {
			logrus.Error(err)
		}
	}

	activeConnection := conf.DefaultConnection
	w, err := NewWLEDConverter(activeConnection, conf.Connections)
	if err == nil {
		err = w.PrepareStore()
		if err != nil {
			logrus.Error(err)
		}
	}
	return &OpWLED{cr: cr, config: &conf}
}

// StartListening (noOp)
func (o *OpWLED) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (o *OpWLED) Shutdown(ctx *base.Context) {
}
