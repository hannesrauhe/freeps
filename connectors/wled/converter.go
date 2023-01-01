package wled

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
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
	x       int
	y       int
	bgcolor color.Color
	dst     *image.RGBA
}

type PixelMatrix struct {
	PixelMatrix [][]string
}

func NewWLEDConverter(x int, y int, bgcolor color.Color) *WLEDConverter {
	w := WLEDConverter{x: x, y: y, dst: image.NewRGBA(image.Rect(0, 0, x, y)), bgcolor: bgcolor}
	return &w
}

func (w *WLEDConverter) SetPixel(x, y int, c color.Color) error {
	if x >= w.x {
		return fmt.Errorf("x dimension out of bounds")
	}
	if y >= w.y {
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

func (w *WLEDConverter) GetJSON(segid int) ([]byte, error) {
	root := WLEDRoot{}
	root.Seg.ID = segid
	root.Seg.I = make([][3]uint32, 0)
	for x := 0; x < w.x; x++ {
		for y := 0; y < w.y; y++ {
			j := y
			if x&1 != 0 {
				j = w.y - y - 1
			}
			r, g, b, _ := w.dst.At(x, j).RGBA()
			p := [3]uint32{r >> 8, g >> 8, b >> 8}
			root.Seg.I = append(root.Seg.I, p)
		}
	}
	return json.Marshal(root)
}

func (w *WLEDConverter) GetPixelMatrix() PixelMatrix {
	root := PixelMatrix{PixelMatrix: make([][]string, 0)}
	for y := 0; y < w.y; y++ {
		root.PixelMatrix = append(root.PixelMatrix, make([]string, w.x))
		for x := 0; x < w.x; x++ {
			c := w.dst.At(x, y)
			root.PixelMatrix[y][x] = utils.GetHexColor(c)
		}
	}
	return root
}

func (w *WLEDConverter) GetImage() *freepsgraph.OperatorIO {
	var bout []byte
	contentType := "image/png"
	writer := bytes.NewBuffer(bout)
	if err := png.Encode(writer, w.dst); err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Encoding to png failed: %v", err.Error())
	}
	return freepsgraph.MakeByteOutputWithContentType(writer.Bytes(), contentType)
}