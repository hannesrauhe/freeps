package wled

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

type OpWLED struct {
	cr     *utils.ConfigReader
	config *OpConfig
	saved  map[string]*WLEDConverter
}

//go:embed font/*
var staticContent embed.FS

var _ freepsgraph.FreepsOperator = &OpWLED{}

// GetName returns the name of the operator
func (o *OpWLED) GetName() string {
	return "wled"
}

func (o *OpWLED) Execute(ctx *utils.Context, function string, vars map[string]string, mainInput *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	c := http.Client{}

	var resp *http.Response
	var err error
	var bgcolor color.Color //TODO: unused

	// TODO: pick a config
	conf := o.config.Connections[o.config.DefaultConnection]
	err = utils.ArgsMapToObject(vars, &conf)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse parameters: %v", err.Error())
	}
	err = conf.Validate()
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Invalid parameters: %v", err.Error())
	}
	w := NewWLEDConverter(conf.Width, conf.Height, bgcolor)

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
		err = w.WriteString(str, c, utils.ParseBool(vars["alignRight"]))
	case "setPixel":
		c := image.White.C
		str, ok := vars["pixelMatrix"]
		if ok {
			wt, ok := o.saved[str]
			if ok {
				w = wt
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
		str, ok := vars["pixelMatrix"]
		if ok {
			wt, ok := o.saved[str]
			if ok {
				w = wt
			}
		}
		return freepsgraph.MakeObjectOutput(w.GetPixelMatrix())
	default:
		return freepsgraph.MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if imgName, ok := vars["pixelMatrix"]; ok {
		o.saved[imgName] = w
	}

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	b, err := w.GetJSON(conf.SegID)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err = c.Post(conf.Address+"/json", "encoding/json", breader)

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}

	if utils.ParseBool(vars["showImage"]) {
		return w.GetImage()
	}
	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &freepsgraph.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: freepsgraph.Byte, ContentType: resp.Header.Get("Content-Type")}
}

func (o *OpWLED) GetFunctions() []string {
	return []string{"setString", "setImage", "setPixel", "getPixelMatrix"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address", "string", "x", "y", "segid", "icon", "color", "bgcolor", "alignRight", "showImage", "pixelMatrix", "height", "width"}
}

func (o *OpWLED) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
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
	return &OpWLED{cr: cr, config: &conf, saved: make(map[string]*WLEDConverter)}
}

// Shutdown (noOp)
func (o *OpWLED) Shutdown(ctx *utils.Context) {
}
