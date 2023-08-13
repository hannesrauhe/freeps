package pixeldisplay

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

type WLEDMatrixDisplayConfig struct {
	Segments              []WLEDSegmentConfig
	Address               string
	MinDisplayDuration    time.Duration
	MaxPictureWidthFactor int
}

type WLEDMatrixDisplay struct {
	segments []*WLEDSegmentRoot
	conf     *WLEDMatrixDisplayConfig
	height   int
	width    int
	lastImg  *image.RGBA
	imgChan  chan image.RGBA
}

var _ Pixeldisplay = &WLEDMatrixDisplay{}

// NewWLEDMatrixDisplay creates a connection to a WLED instance with multiple segments
func NewWLEDMatrixDisplay(cfg WLEDMatrixDisplayConfig) (*WLEDMatrixDisplay, error) {
	disp := &WLEDMatrixDisplay{conf: &cfg}
	for _, segCfg := range cfg.Segments {
		seg, err := newWLEDSegmentRoot(segCfg)
		if err != nil {
			return nil, err
		}
		disp.segments = append(disp.segments, seg)
		if disp.height < segCfg.Height+segCfg.OffsetY {
			disp.height = segCfg.Height + segCfg.OffsetY
		}
		if disp.width < segCfg.Width+segCfg.OffsetX {
			disp.width = segCfg.Width + segCfg.OffsetX
		}
	}
	disp.imgChan = make(chan image.RGBA, cfg.MaxPictureWidthFactor*disp.width)
	go disp.drawLoop(disp.imgChan, cfg.MinDisplayDuration)
	return disp, nil
}

func (d *WLEDMatrixDisplay) sendCmd(cmd *base.OperatorIO) *base.OperatorIO {
	c := http.Client{}

	var b []byte
	var err error
	path := d.conf.Address + "/json/state"
	b, err = cmd.GetBytes()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err := c.Post(path, "application/json", breader)

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}

	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &base.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: base.Byte, ContentType: resp.Header.Get("Content-Type")}
}

func (d *WLEDMatrixDisplay) Shutdown() {
	close(d.imgChan)
}

func (d *WLEDMatrixDisplay) drawImageImmediately(dst *image.RGBA) *base.OperatorIO {
	for _, seg := range d.segments {
		err := seg.SendToWLEDSegment(d.conf.Address, *dst)
		if err.IsError() {
			return err
		}
	}
	return base.MakeEmptyOutput()
}

func (d *WLEDMatrixDisplay) DrawImage(img image.Image, returnPNG bool) *base.OperatorIO {
	b := image.Rect(0, 0, d.width, d.height)
	converted := image.NewRGBA(b)
	draw.Draw(converted, b, img, b.Min, draw.Src)
	d.lastImg = converted

	d.imgChan <- *converted
	if !returnPNG {
		return base.MakeEmptyOutput()
	}
	var bout []byte
	contentType := "image/png"
	writer := bytes.NewBuffer(bout)
	if err := png.Encode(writer, converted); err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Encoding to png failed: %v", err.Error())
	}
	return base.MakeByteOutputWithContentType(writer.Bytes(), contentType)
}

func (d *WLEDMatrixDisplay) SetBrightness(brightness int) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) SetColor(color color.Color) *base.OperatorIO {
	return base.MakeEmptyOutput()
}

func (d *WLEDMatrixDisplay) SetBackgroundColor(color color.Color) *base.OperatorIO {
	return base.MakeEmptyOutput()
}

func (d *WLEDMatrixDisplay) DrawPixel(x, y int, color color.Color) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) TurnOn() *base.OperatorIO {
	return d.sendCmd(base.MakeObjectOutput(&WLEDState{On: true}))
}

func (d *WLEDMatrixDisplay) TurnOff() *base.OperatorIO {
	return d.sendCmd(base.MakeObjectOutput(&WLEDState{On: false}))
}

func (d *WLEDMatrixDisplay) GetDimensions() image.Point {
	return image.Point{X: d.width, Y: d.height}
}

func (d *WLEDMatrixDisplay) GetMaxPictureSize() image.Point {
	return image.Point{X: d.width * d.conf.MaxPictureWidthFactor, Y: d.height}
}

func (d *WLEDMatrixDisplay) GetColor() color.Color {
	return color.White
}

func (d *WLEDMatrixDisplay) GetBackgroundColor() color.Color {
	return color.Black
}

func (d *WLEDMatrixDisplay) GetText() string {
	return ""
}

func (d *WLEDMatrixDisplay) GetImage() *image.RGBA {
	return d.lastImg
}

func (d *WLEDMatrixDisplay) IsOn() bool {
	return true
}

// drawLoop starts a loop that draws an image from a channel to the display and then sleeps for the given duration
func (d *WLEDMatrixDisplay) drawLoop(c <-chan image.RGBA, duration time.Duration) {
	for {
		img, ok := <-c
		if !ok {
			return
		}
		d.drawImageImmediately(&img)
		time.Sleep(duration)
	}
}
