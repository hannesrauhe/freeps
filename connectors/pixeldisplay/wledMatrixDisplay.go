package pixeldisplay

import (
	"bytes"
	"image"
	"io"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

type WLEDSegmentConfig struct {
	Width  int `json:",string"`
	Height int `json:",string"`
	SegID  int `json:",string"`
}

type WLEDMatrixDisplayConfig struct {
	Segments []WLEDSegmentConfig
	Address  string
}

type WLEDMatrixDisplay struct {
	segments []WLEDSegment
	conf     *WLEDMatrixDisplayConfig
}

var _ Pixeldisplay = &WLEDMatrixDisplay{}

// NewWLEDMatrixDisplay creates a connection to a WLED instance with multiple segments
func NewWLEDMatrixDisplay(cfg WLEDMatrixDisplayConfig) (*WLEDMatrixDisplay, error) {
	disp := &WLEDMatrixDisplay{conf: &cfg}
	// for _, segCfg := range cfg.segments {
	// 	seg, err := NewWLEDSegment(segCfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	disp.segments = append(disp.segments, seg)
	// }
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
}

func (d *WLEDMatrixDisplay) Width() int {
	return d.conf.Segments[0].Width
}

func (d *WLEDMatrixDisplay) Height() int {
	return d.conf.Segments[0].Height
}

func (d *WLEDMatrixDisplay) SetImage(dst *image.RGBA) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) SetBrightness(brightness int) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) SetText(text string) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) SetColor(color string) *base.OperatorIO {

	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) SetBackground(color string) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) SetPixel(x, y int, color string) *base.OperatorIO {
	return d.sendCmd(nil)
}

func (d *WLEDMatrixDisplay) TurnOn() *base.OperatorIO {
	return d.sendCmd(base.MakeObjectOutput(&WLEDState{On: true}))
}

func (d *WLEDMatrixDisplay) TurnOff() *base.OperatorIO {
	return d.sendCmd(base.MakeObjectOutput(&WLEDState{On: false}))
}

func (d *WLEDMatrixDisplay) GetDimensions() (width, height int) {
	return d.conf.Segments[0].Width, d.conf.Segments[0].Height
}

func (d *WLEDMatrixDisplay) GetColor() string {
	return ""
}

func (d *WLEDMatrixDisplay) GetBackground() string {
	return ""
}

func (d *WLEDMatrixDisplay) GetText() string {
	return ""
}

func (d *WLEDMatrixDisplay) GetImage() *image.RGBA {
	return nil
}

func (d *WLEDMatrixDisplay) IsOn() bool {
	return true
}
