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
)

type OpWLED struct {
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
	var bgcolor color.Color

	x, err := strconv.Atoi(vars["x"])
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "x not a valid integer")
	}
	y, err := strconv.Atoi(vars["y"])
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "y not a valid integer")
	}

	if colstr, ok := vars["bgcolor"]; ok {
		bgcolor, err = utils.ParseHexColor(colstr)
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "color not a valid hex color")
		}
	}
	w := NewWLEDConverter(x, y, bgcolor)

	segid := 0
	if _, ok := vars["segid"]; ok {
		segid, err = strconv.Atoi(vars["segid"])
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "segid not a valid integer")
		}
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
		err = w.WriteString(str, c, utils.ParseBool(vars["alignRight"]))
	default:
		return freepsgraph.MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	b, err := w.GetJSON(segid)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err = c.Post(vars["address"]+"/json", "encoding/json", breader)

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
	return []string{"setString", "setImage"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address", "string", "x", "y", "segid", "icon", "color", "bgcolor", "alignRight", "showImage"}
}

func (o *OpWLED) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpWLED) Shutdown(ctx *utils.Context) {
}
