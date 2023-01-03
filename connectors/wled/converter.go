package wled

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

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type WLEDSegment struct {
	ID int         `json:"id"`
	I  [][3]uint32 `json:"i"`
}

type WLEDRoot struct {
	Seg WLEDSegment `json:"seg"`
}

type WLEDConverter struct {
	conf WLEDConfig
	dst  *image.RGBA
}

func NewWLEDConverter(conf WLEDConfig) *WLEDConverter {
	w := WLEDConverter{conf: conf, dst: image.NewRGBA(image.Rect(0, 0, conf.Width, conf.Height))}
	return &w
}

func (w *WLEDConverter) SetPixel(x, y int, c color.Color) error {
	if x >= w.conf.Width {
		return fmt.Errorf("x dimension out of bounds")
	}
	if y >= w.conf.Height {
		return fmt.Errorf("y dimension out of bounds")
	}
	w.dst.Set(x, y, c)
	return nil
}

func (w *WLEDConverter) WriteString(s string, c color.Color, alignRight bool) error {
	const (
		width        = 16
		height       = 8
		startingDotX = 1
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
	if alignRight {
		endDot := d.MeasureString(s)
		toMove := width - endDot.Ceil()
		if toMove > 0 {
			d.Dot = fixed.P(startingDotX+toMove, startingDotY)
		}
	}
	d.DrawString(s)
	return nil
}

func (w *WLEDConverter) ScaleImage(src image.Image) {
	draw.NearestNeighbor.Scale(w.dst, w.dst.Rect, src, src.Bounds(), draw.Over, nil)
}

func (w *WLEDConverter) GetJSON() ([]byte, error) {
	root := WLEDRoot{}
	root.Seg.ID = w.conf.SegID
	root.Seg.I = make([][3]uint32, 0)
	for x := 0; x < w.conf.Width; x++ {
		for y := 0; y < w.conf.Height; y++ {
			j := y
			if x&1 != 0 {
				j = w.conf.Height - y - 1
			}
			r, g, b, _ := w.dst.At(x, j).RGBA()
			p := [3]uint32{r >> 8, g >> 8, b >> 8}
			root.Seg.I = append(root.Seg.I, p)
		}
	}
	return json.Marshal(root)
}

func (w *WLEDConverter) GetPixelMatrix() PixelMatrix {
	pm := make([][]string, 0)
	for y := 0; y < w.conf.Height; y++ {
		pm = append(pm, make([]string, w.conf.Width))
		for x := 0; x < w.conf.Width; x++ {
			c := w.dst.At(x, y)
			pm[y][x] = utils.GetHexColor(c)
		}
	}
	return pm
}

func (w *WLEDConverter) SetPixelMatrix(pm PixelMatrix) error {
	for y := 0; y < w.conf.Height; y++ {
		pm = append(pm, make([]string, w.conf.Width))
		for x := 0; x < w.conf.Width; x++ {
			p, err := utils.ParseHexColor(pm[y][x])
			if err != nil {
				return fmt.Errorf("Pixel %v,%v: %v", x, y, err.Error())
			}
			w.dst.Set(x, y, p)
		}
	}
	return nil
}

func (w *WLEDConverter) GetPNG() *freepsgraph.OperatorIO {
	var bout []byte
	contentType := "image/png"
	writer := bytes.NewBuffer(bout)
	if err := png.Encode(writer, w.dst); err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Encoding to png failed: %v", err.Error())
	}
	return freepsgraph.MakeByteOutputWithContentType(writer.Bytes(), contentType)
}

func (w *WLEDConverter) SendToWLED(returnPNG bool) *freepsgraph.OperatorIO {
	c := http.Client{}

	b, err := w.GetJSON()
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err := c.Post(w.conf.Address+"/json", "encoding/json", breader)

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}

	if returnPNG {
		return w.GetPNG()
	}
	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &freepsgraph.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: freepsgraph.Byte, ContentType: resp.Header.Get("Content-Type")}
}
