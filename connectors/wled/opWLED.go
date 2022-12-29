package wled

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
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

	segid, err := strconv.Atoi(vars["segid"])
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "segid not a valid integer")
	}

	switch function {
	case "setImage":
		var binput []byte
		var contentType string
		var img image.Image

		if vars["icon"] != "" {
			binput, err = staticContent.ReadFile("font/" + vars["icon"] + ".png")
			contentType = "image/png"
		} else if !mainInput.IsEmpty() {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "need image as Input")
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
		err = w.WriteString(str, c)
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
	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &freepsgraph.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: freepsgraph.Byte, ContentType: resp.Header.Get("Content-Type")}
}

func (o *OpWLED) GetFunctions() []string {
	return []string{"setString", "setImage"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address", "string", "x", "y", "segid", "icon", "color", "bgcolor"}
}

func (o *OpWLED) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpWLED) Shutdown(ctx *utils.Context) {
}

type WLEDSegment struct {
	ID int         `json:"id"`
	I  [][3]uint32 `json:"i"`
}

type WLEDRoot struct {
	Seg WLEDSegment `json:"seg"`
}

type WLEDConverter struct {
	r       WLEDRoot
	x       int
	y       int
	bgcolor color.Color
	dst     *image.RGBA
}

func NewWLEDConverter(x int, y int, bgcolor color.Color) *WLEDConverter {
	w := WLEDConverter{x: x, y: y, dst: image.NewRGBA(image.Rect(0, 0, x, y)), bgcolor: bgcolor}
	return &w
}

func (w *WLEDConverter) AppendIndividualPixel(r uint32, g uint32, b uint32) {
	if w.r.Seg.I == nil {
		w.r.Seg.I = make([][3]uint32, 0)
	}
	w.r.Seg.I = append(w.r.Seg.I, [3]uint32{r, g, b})
}

func (w *WLEDConverter) SetPixel(x, y int, r, g, b uint8) error {
	if x >= w.x {
		return fmt.Errorf("x dimension out of bounds")
	}
	if y >= w.y {
		return fmt.Errorf("y dimension out of bounds")
	}
	w.dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b})
	return nil
}

func (w *WLEDConverter) WriteString(s string, c color.Color) error {
	const (
		width        = 16
		height       = 8
		startingDotX = 2
		startingDotY = 7
	)

	fontBytes, err := staticContent.ReadFile("font/Grand9K Pixel.ttf")
	if err != nil {
		log.Fatalf("Reading file from embed fs: %v", err)
	}
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    32,
		DPI:     18,
		Hinting: font.HintingNone,
	})
	if err != nil {
		log.Fatalf("NewFace: %v", err)
	}

	d := font.Drawer{
		Dst:  w.dst,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(startingDotX, startingDotY),
	}
	d.DrawString(s)
	return nil
}

func (w *WLEDConverter) ScaleImage(src image.Image) {
	draw.NearestNeighbor.Scale(w.dst, w.dst.Rect, src, src.Bounds(), draw.Over, nil)
}

func (w *WLEDConverter) GetJSON(segid int) ([]byte, error) {
	if w.r.Seg.I == nil {
		w.r.Seg.ID = segid
		w.r.Seg.I = make([][3]uint32, 0)
	}
	for x := 0; x < w.x; x++ {
		for y := 0; y < w.y; y++ {
			j := y
			if x&1 != 0 {
				j = w.y - y - 1
			}
			r, g, b, _ := w.dst.At(x, j).RGBA()
			p := [3]uint32{r >> 8, g >> 8, b >> 8}
			w.r.Seg.I = append(w.r.Seg.I, p)
		}
	}
	return json.Marshal(w.r)
}

func (w *WLEDConverter) GetImage() *freepsgraph.OperatorIO {
	var bout []byte
	contentType := "image/png"
	writer := bytes.NewBuffer(bout)
	if err := png.Encode(writer, w.dst); err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Encoding to png failed: %v", err.Error())
	}
	return freepsgraph.MakeByteOutputWithContentType(bout, contentType)
}
