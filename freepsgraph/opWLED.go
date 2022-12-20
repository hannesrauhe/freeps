package freepsgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/utils"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type OpWLED struct {
}

var _ FreepsOperator = &OpWLED{}

// GetName returns the name of the operator
func (o *OpWLED) GetName() string {
	return "wled"
}

func (o *OpWLED) Execute(ctx *utils.Context, function string, vars map[string]string, mainInput *OperatorIO) *OperatorIO {
	c := http.Client{}

	var resp *http.Response
	var err error

	x, err := strconv.Atoi(vars["x"])
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "x not a valid integer")
	}
	y, err := strconv.Atoi(vars["y"])
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "y not a valid integer")
	}
	w := NewWLEDConverter(x, y)

	segid, err := strconv.Atoi(vars["segid"])
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "segid not a valid integer")
	}

	switch function {
	case "setImage":
		if mainInput.IsEmpty() {
			return MakeOutputError(http.StatusBadRequest, "need image as Input")
		}
		binput, err := mainInput.GetBytes()
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		var img image.Image

		ctx.GetLogger().Debugf("Decoding image of type: %v", mainInput.ContentType)
		if mainInput.ContentType == "image/png" {
			img, err = png.Decode(bytes.NewReader(binput))
		} else {
			img, _, err = image.Decode(bytes.NewReader(binput))
		}
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		w.ScaleImage(img)
	case "setString":
		str, ok := vars["string"]
		if !ok {
			str = mainInput.GetString()
		}
		err = w.WriteString(str)
	default:
		return MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}

	b, err := w.GetJSON(segid)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err = c.Post(vars["address"]+"/json", "encoding/json", breader)

	if err != nil {
		return MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: Byte, ContentType: resp.Header.Get("Content-Type")}
}

func (o *OpWLED) GetFunctions() []string {
	return []string{"setString", "setImage"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address", "string", "x", "y", "segid"}
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
	r   WLEDRoot
	x   int
	y   int
	dst *image.RGBA
}

func NewWLEDConverter(x int, y int) *WLEDConverter {
	w := WLEDConverter{x: x, y: y, dst: image.NewRGBA(image.Rect(0, 0, x, y))}
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

func (w *WLEDConverter) WriteString(s string) error {
	const (
		width        = 16
		height       = 8
		startingDotX = 2
		startingDotY = 7
	)

	f, err := opentype.Parse(gomono.TTF)
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
		Src:  image.White,
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
