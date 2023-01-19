package wled

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

type OpWLED struct {
	cr     *utils.ConfigReader
	config *OpConfig
	saved  map[string]PixelMatrix
}

//go:embed font/*
var staticContent embed.FS

var _ freepsgraph.FreepsOperator = &OpWLED{}

// GetName returns the name of the operator
func (o *OpWLED) GetName() string {
	return "wled"
}

func (o *OpWLED) Execute(ctx *utils.Context, function string, vars map[string]string, mainInput *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	var err error

	// TODO: pick a config
	conf := o.config.Connections[o.config.DefaultConnection]
	err = utils.ArgsMapToObject(vars, &conf)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse parameters: %v", err.Error())
	}
	w, err := NewWLEDConverter(conf, o.config.Connections)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Invalid parameters: %v", err.Error())
	}

	var pm struct {
		PixelMatrix [][]string
		Name        string
		NextColor   string
		Segment     string
	}

	switch function {
	case "setImage":
		var binput []byte
		var contentType string
		var img image.Image

		if vars["icon"] != "" {
			binput, err = staticContent.ReadFile("font/" + vars["icon"] + ".png")
			contentType = "image/png"
		} else if mainInput.IsEmpty() {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "no input, expecting an image")
		} else {
			binput, err = mainInput.GetBytes()
			if err != nil {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
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
			return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
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
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "color not a valid hex color")
			}
		}
		err = w.WriteStringPf(str, c)
	case "setPixel":
		c := image.White.C
		str, ok := vars["pixelMatrix"]
		if ok {
			wt, ok := o.saved[str]
			if ok {
				w.SetPixelMatrix(wt)
			}
		}
		if colstr, ok := vars["color"]; ok {
			c, err = utils.ParseHexColor(colstr)
			if err != nil {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "color not a valid hex color")
			}
		}
		x, err := strconv.Atoi(vars["x"])
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "x not a valid integer")
		}
		y, err := strconv.Atoi(vars["y"])
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "y not a valid integer")
		}
		err = w.SetPixel(x, y, c)
	case "getPixelMatrix":
		pmName, ok := vars["pixelMatrix"]
		if !ok || pmName == "" {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "pixelMatrix paramter should contain the name but is missing")
		}
		wt, ok := o.saved[pmName]
		if ok {
			w.SetPixelMatrix(wt)
		}
		pm.PixelMatrix = w.GetPixelMatrix()
		pm.Name = pmName
		pm.NextColor = vars["color"]
		pm.Segment = vars["SegID"]
		return freepsgraph.MakeObjectOutput(pm)
	case "setPixelMatrix":
		pmName := vars["pixelMatrix"]
		if !mainInput.IsEmpty() {
			err := mainInput.ParseJSON(&pm)
			if err != nil {
				return freepsgraph.MakeOutputError(http.StatusNotFound, "Could not parse input as pixelmatrix object: %v", err)
			}
			if pmName == "" {
				pmName = pm.Name
			}
			if pmName == "" {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "pixelMatrix name should be given either via parameter or input, but it's empty in both")
			}
			o.saved[pmName] = pm.PixelMatrix
		}
		animate := AnimationOptions{StepDuration: time.Millisecond * 500}
		err := utils.ArgsMapToObject(vars, &animate)
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusNotFound, "Could not parse Animation Parameters: %v", err)
		}
		return o.SetPixelMatrix(w, pmName, animate)
	default:
		return freepsgraph.MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if pmName, ok := vars["pixelMatrix"]; ok {
		o.saved[pmName] = w.GetPixelMatrix()
	} else {
		o.saved["last"] = w.GetPixelMatrix()
	}

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	return w.SendToWLED(utils.ParseBool(vars["showImage"]))
}

func (o *OpWLED) GetFunctions() []string {
	return []string{"setString", "setImage", "setPixel", "getPixelMatrix", "setPixelMatrix"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address", "string", "x", "y", "segid", "icon", "color", "bgcolor", "alignRight", "showImage", "pixelMatrix", "height", "width", "animationType"}
}

func (o *OpWLED) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "animationType":
		return map[string]string{"move": "move", "shift": "shift", "squence": "sequence"}
	case "showImage", "alignRight":
		return map[string]string{"true": "true", "false": "false"}
	case "pixelMatrix":
		m := map[string]string{}
		for k, _ := range o.saved {
			m[k] = k
		}
		return m
	}
	return map[string]string{}
}

type AnimationOptions struct {
	AnimationType string
	StepDuration  time.Duration
	Repeat        int `json:",string"`
}

func (o *OpWLED) SetPixelMatrix(w *WLEDConverter, pmName string, animate AnimationOptions) *freepsgraph.OperatorIO {
	for r := 0; r <= animate.Repeat; r++ {
		pm, ok := o.saved[pmName]
		if !ok {
			if pmName == "diagonal" {
				pm = MakeDiagonalPixelMatrix(w.Width(), w.Height(), "#FF0000", "#000000")
			} else if pmName == "zigzag" {
				pm = MakeZigZagPixelMatrix(w.Width(), w.Height(), "#FF0000", "#000000")
			} else {
				return freepsgraph.MakeOutputError(404, "No such Pixel Matrix \"%v\"", pmName)
			}
		}
		switch animate.AnimationType {
		case "move":
			for i := -1 * len(pm[0]); i < len(pm[0]); i++ {
				wt := pm.MoveRight("#000000", i)
				w.SetPixelMatrix(wt)
				w.SendToWLED(false)
				time.Sleep(animate.StepDuration)
			}
		case "shift":
			for i := 0; i < len(pm[0]); i++ {
				wt := pm.Shift(i)
				w.SetPixelMatrix(wt)
				w.SendToWLED(false)
				time.Sleep(animate.StepDuration)
			}
		case "sequence":
			for i := 1; ok; i++ {
				w.SetPixelMatrix(pm)
				w.SendToWLED(false)
				time.Sleep(animate.StepDuration)
				pm, ok = o.saved[fmt.Sprintf("%v.%d", pmName, i)]
			}
		default:
			w.SetPixelMatrix(pm)
			w.SendToWLED(false)
			return freepsgraph.MakeEmptyOutput()
		}
	}
	return freepsgraph.MakeEmptyOutput()
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
	return &OpWLED{cr: cr, config: &conf, saved: make(map[string]PixelMatrix)}
}

// Shutdown (noOp)
func (o *OpWLED) Shutdown(ctx *utils.Context) {
}
