package wled

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"path"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed font/* pixelart/*
var staticContent embed.FS

type WLEDConverter struct {
	segments []WLEDRoot
	dst      *image.RGBA
}

// NewWLEDConverter creates a connection to one or multiple WLED instances, the config might reference other configs
func NewWLEDConverter(confName string, connections map[string]WLEDConfig) (*WLEDConverter, error) {
	conf, exists := connections[confName]
	if !exists {
		return nil, fmt.Errorf("Connection \"%v\" does not exist", confName)
	}
	err := conf.Validate(false)
	if err != nil {
		return nil, err
	}
	segments := []WLEDRoot{}
	if conf.References == nil || len(conf.References) == 0 {
		newSeg, err := newWLEDRoot(conf, 0, 0)
		if err != nil {
			return nil, err
		}
		segments = append(segments, *newSeg)
	} else {
		for _, r := range conf.References {
			subconf, ok := connections[r.Name]
			if !ok {
				return nil, fmt.Errorf("Unknown referenced connection: %v", r.Name)
			}
			newSeg, err := newWLEDRoot(subconf, r.OffsetX, r.OffsetY)
			if err != nil {
				return nil, fmt.Errorf("Unknown when adding referenced connection \"%v\": %v", r.Name, err)
			}
			segments = append(segments, *newSeg)
		}
	}
	w := WLEDConverter{segments: segments}
	w.dst = image.NewRGBA(image.Rect(0, 0, w.Width(), w.Height()))
	return &w, nil
}

func (w *WLEDConverter) Width() int {
	max := int(0)
	for _, s := range w.segments {
		if max < s.offsetX+s.Width() {
			max = s.offsetX + s.Width()
		}
	}
	return max
}

func (w *WLEDConverter) Height() int {
	max := int(0)
	for _, s := range w.segments {
		if max < s.offsetY+s.Height() {
			max = s.offsetY + s.Height()
		}
	}
	return max
}

func (w *WLEDConverter) SetPixel(x, y int, c color.Color) error {
	if x >= w.dst.Rect.Dx() {
		return fmt.Errorf("x dimension out of bounds")
	}
	if y >= w.dst.Rect.Dy() {
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
		return fmt.Errorf("Reading file from embed fs: %v", err)
	}
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    32,
		DPI:     18,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return fmt.Errorf("NewFace: %v", err)
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

func (w *WLEDConverter) GetPixelMatrix() PixelMatrix {
	pm := make([][]string, 0)
	for y := 0; y < w.Height(); y++ {
		pm = append(pm, make([]string, w.Width()))
		for x := 0; x < w.Width(); x++ {
			c := w.dst.At(x, y)
			pm[y][x] = utils.GetHexColor(c)
		}
	}
	return pm
}

func (w *WLEDConverter) DrawPixelMatrix(pm PixelMatrix) error {
	for y := 0; y < w.Height() && y < len(pm); y++ {
		for x := 0; x < w.Width() && x < len(pm[y]); x++ {
			p, err := utils.ParseHexColor(pm[y][x])
			if err != nil {
				return fmt.Errorf("Pixel %v,%v: %v", x, y, err.Error())
			}
			w.dst.Set(x, y, p)
		}
	}
	return nil
}

func (w *WLEDConverter) SetPixelMatrix(pmName string) error {
	wledNs := freepsstore.GetGlobalStore().GetNamespace("_wled")
	io := wledNs.GetValue(pmName)
	if io.IsError() {
		return fmt.Errorf("No pixelmatrix stored")
	}
	var pm PixelMatrix
	err := io.ParseJSON(&pm)
	if err != nil {
		return fmt.Errorf("Could not parse input as pixelmatrix object: %v", err)
	}
	return w.DrawPixelMatrix(pm)
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

// SendToWLED sends a command to WLED, if cmd is nil, it sends the stored picture instead
func (w *WLEDConverter) SendToWLED(cmd *freepsgraph.OperatorIO, returnPNG bool) *freepsgraph.OperatorIO {
	resp := freepsgraph.MakeEmptyOutput()
	overallResp := freepsgraph.MakeEmptyOutput()
	for i, s := range w.segments {
		if cmd == nil {
			resp = s.SendToWLED(cmd, w.dst)
		} else {
			resp = s.SendToWLED(cmd, nil)
		}
		if resp.IsError() {
			if len(w.segments) == 1 {
				return resp
			}
			overallResp = freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error in segment %v: %v", i, resp.GetString())
			return overallResp
		}
		overallResp = resp
	}

	if cmd == nil && returnPNG {
		return w.GetPNG()
	}
	return overallResp
}

func (w *WLEDConverter) StorePixelMatrix(ctx *base.Context, pmName string) error {
	wledNs := freepsstore.GetGlobalStore().GetNamespace("_wled")
	return wledNs.SetValue(pmName, freepsgraph.MakeObjectOutput(w.GetPixelMatrix()), ctx.GetID())
}

func (w *WLEDConverter) PrepareStore() error {
	wledNs := freepsstore.GetGlobalStore().GetNamespace("_wled")
	wledNs.SetValue("last", freepsgraph.MakeObjectOutput(w.GetPixelMatrix()), "startup")
	wledNs.SetValue("diagonal", freepsgraph.MakeObjectOutput(MakeDiagonalPixelMatrix(w.Width(), w.Height(), "#FF0000", "#000000")), "startup")
	wledNs.SetValue("zigzag", freepsgraph.MakeObjectOutput(MakeZigZagPixelMatrix(w.Width(), w.Height(), "#FF0000", "#000000")), "startup")
	files, err := staticContent.ReadDir("pixelart")
	if err != nil {
		return err
	}
	retErr := err
	for _, fn := range files {
		key := fn.Name()[:len(fn.Name())-5]
		value, err := staticContent.ReadFile(path.Join("pixelart", fn.Name()))
		if err != nil {
			retErr = err
			continue
		}
		var pm PixelMatrix
		err = json.Unmarshal(value, &pm)
		if err != nil {
			retErr = err
			continue
		}

		wledNs.SetValue(key, freepsgraph.MakeObjectOutput(pm), "startup")
	}
	return retErr
}
