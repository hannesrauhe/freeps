package pixeldisplay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

type ImageWithMetadata struct {
	Image   image.RGBA
	Created time.Time
	Ctx     *base.Context
}

type WLEDMatrixDisplayConfig struct {
	Segments              []WLEDSegmentConfig
	Address               string
	MinDisplayDuration    time.Duration
	MaxPictureWidthFactor int
}

type WLEDMatrixDisplay struct {
	segments            map[int]*WLEDSegmentHolder
	conf                *WLEDMatrixDisplayConfig
	height              int
	width               int
	backgroundLayerLock sync.Mutex
	backgroundLayer     map[string]image.RGBA
	imgChan             chan ImageWithMetadata
	color               color.Color
	bgColor             color.Color
}

type WLEDSegmentResponse struct {
	ID    int `json:"id"`
	Start int `json:"start,omitempty"`
	Stop  int `json:"stop,omitempty"`
	Len   int `json:"len,omitempty"`
}

type WLEDResponse struct {
	Seg        []WLEDSegmentResponse `json:"seg,omitempty"`
	On         bool                  `json:"on"`
	Brightness int                   `json:"bri"`
}

var _ Pixeldisplay = &WLEDMatrixDisplay{}

// NewWLEDMatrixDisplay creates a connection to a WLED instance with multiple segments
func NewWLEDMatrixDisplay(cfg WLEDMatrixDisplayConfig) (*WLEDMatrixDisplay, error) {
	disp := &WLEDMatrixDisplay{conf: &cfg, color: color.White, bgColor: color.Transparent, segments: map[int]*WLEDSegmentHolder{}, backgroundLayer: make(map[string]image.RGBA), backgroundLayerLock: sync.Mutex{}}
	for _, segCfg := range cfg.Segments {
		seg, err := newWLEDSegmentHolder(segCfg)
		if err != nil {
			return nil, err
		}
		disp.segments[segCfg.SegID] = seg
		if disp.height < segCfg.Height+segCfg.OffsetY {
			disp.height = segCfg.Height + segCfg.OffsetY
		}
		if disp.width < segCfg.Width+segCfg.OffsetX {
			disp.width = segCfg.Width + segCfg.OffsetX
		}
	}
	disp.imgChan = make(chan ImageWithMetadata, cfg.MaxPictureWidthFactor)
	go disp.drawLoop(disp.imgChan, cfg.MinDisplayDuration)
	return disp, nil
}

func (d *WLEDMatrixDisplay) getState() (WLEDResponse, *base.OperatorIO) {
	var state WLEDResponse
	c := http.Client{}

	var err error
	path := d.conf.Address + "/json/state"
	resp, err := c.Get(path)

	if err != nil {
		return state, base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}

	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	res := base.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: base.Byte, ContentType: resp.Header.Get("Content-Type")}
	if res.IsError() {
		return state, &res
	}
	err = res.ParseJSON(&state)
	if err != nil {
		return state, base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	return state, &res
}

func (d *WLEDMatrixDisplay) sendCmd(file string, cmd *base.OperatorIO) *base.OperatorIO {
	c := http.Client{}

	var b []byte
	var err error
	path := d.conf.Address + "/json/" + file
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
	d.backgroundLayerLock.Lock()
	defer d.backgroundLayerLock.Unlock()
	for _, seg := range d.segments {
		err := seg.SendToWLEDSegment(d.conf.Address, *dst, d.backgroundLayer)
		if err.IsError() {
			return err
		}
	}
	return base.MakeEmptyOutput()
}

func (d *WLEDMatrixDisplay) DrawImage(ctx *base.Context, img image.Image, returnPNG bool) *base.OperatorIO {
	if img == nil {
		return base.MakeOutputError(http.StatusBadRequest, "no image to draw")
	}
	b := image.Rect(0, 0, d.width, d.height)
	converted := image.NewRGBA(b)
	draw.Draw(converted, b, img, b.Min, draw.Src)

	d.imgChan <- ImageWithMetadata{Image: *converted, Created: time.Now(), Ctx: ctx}
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

func (d *WLEDMatrixDisplay) SetBackgroundLayer(ctx *base.Context, img image.Image, layerName string) {
	d.backgroundLayerLock.Lock()
	defer d.backgroundLayerLock.Unlock()
	if img == nil {
		delete(d.backgroundLayer, layerName)
		return
	}

	b := image.Rect(0, 0, d.width, d.height)
	converted := image.NewRGBA(b)
	draw.Draw(converted, b, img, b.Min, draw.Src)
	d.backgroundLayer[layerName] = *converted
}

func (d *WLEDMatrixDisplay) ResetBackground(ctx *base.Context) {
	d.backgroundLayerLock.Lock()
	defer d.backgroundLayerLock.Unlock()
	d.backgroundLayer = map[string]image.RGBA{}
}

func (d *WLEDMatrixDisplay) GetBackgroundLayerNames() []string {
	d.backgroundLayerLock.Lock()
	defer d.backgroundLayerLock.Unlock()
	keys := make([]string, 0, len(d.backgroundLayer))
	for k := range d.backgroundLayer {
		keys = append(keys, k)
	}
	return keys
}

func (d *WLEDMatrixDisplay) SetEffect(fx int) *base.OperatorIO {
	cmd := fmt.Sprintf("{\"seg\":{\"fx\":%d},\"v\":true}", fx)
	return d.sendCmd("si", base.MakeByteOutput([]byte(cmd)))
}

func (d *WLEDMatrixDisplay) SetBrightness(brightness int) *base.OperatorIO {
	cmd := fmt.Sprintf("{\"bri\":%d,\"v\":true}", brightness)
	return d.sendCmd("si", base.MakeByteOutput([]byte(cmd)))
}

func (d *WLEDMatrixDisplay) SetColor(color color.Color) {
	d.color = color
}

func (d *WLEDMatrixDisplay) SetBackgroundColor(color color.Color) {
	d.bgColor = color
}

func (d *WLEDMatrixDisplay) DrawPixel(x, y int, color color.Color) *base.OperatorIO {
	return d.sendCmd("state", nil)
}

func (d *WLEDMatrixDisplay) TurnOn() *base.OperatorIO {
	s, err := d.getState()
	if err.IsError() {
		return err
	}
	cmdOutput := d.sendCmd("state", base.MakeObjectOutput(&WLEDResponse{On: true}))
	if cmdOutput.IsError() {
		return cmdOutput
	}
	/* validation */
	returnMap := map[string][]string{}
	returnMap["warnings"] = []string{}
	for _, actualSeg := range s.Seg {
		expectedSeg, exists := d.segments[actualSeg.ID]
		if !exists {
			returnMap["warnings"] = append(returnMap["warnings"], fmt.Sprintf("Segment %v is not configured", actualSeg.ID))
			continue
		}
		if expectedSeg.conf.Height*expectedSeg.conf.Width != actualSeg.Len {
			returnMap["warnings"] = append(returnMap["warnings"], fmt.Sprintf("Segment %v has a length of %v, but expected dimensions are %vx%v (length %v)", expectedSeg.conf.SegID, actualSeg.Len, expectedSeg.conf.Width, expectedSeg.conf.Height, expectedSeg.conf.Height*expectedSeg.conf.Width))
		}
		d.segments[actualSeg.ID].actualLen = actualSeg.Len
	}
	if len(returnMap["warnings"]) > 0 {
		return base.MakeObjectOutput(returnMap)
	}
	return cmdOutput
}

func (d *WLEDMatrixDisplay) TurnOff() *base.OperatorIO {
	return d.sendCmd("state", base.MakeObjectOutput(&WLEDResponse{On: false}))
}

func (d *WLEDMatrixDisplay) GetDimensions() image.Point {
	return image.Point{X: d.width, Y: d.height}
}

func (d *WLEDMatrixDisplay) GetMaxPictureSize() image.Point {
	return image.Point{X: d.width * d.conf.MaxPictureWidthFactor, Y: d.height}
}

func (d *WLEDMatrixDisplay) GetColor() color.Color {
	return d.color
}

func (d *WLEDMatrixDisplay) GetBackgroundColor() color.Color {
	return d.bgColor
}

func (d *WLEDMatrixDisplay) GetBrightness() int {
	state, res := d.getState()
	if res.IsError() {
		return -1
	}
	return state.Brightness
}

func (d *WLEDMatrixDisplay) IsOn() bool {
	state, res := d.getState()
	if res.IsError() {
		return false
	}
	return state.On
}

// drawLoop starts a loop that draws an image from a channel to the display and then sleeps for the given duration
func (d *WLEDMatrixDisplay) drawLoop(c <-chan ImageWithMetadata, waitDuration time.Duration) {
	timeoutDuration := 2 * time.Minute
	for {
		img, ok := <-c
		if !ok {
			return
		}
		delay := time.Now().Sub(img.Created)
		if delay > timeoutDuration {
			img.Ctx.GetLogger().Errorf("Timeout when drawing to Pixeldisplay, delay is: %s", delay)
			continue
		}

		start := time.Now()
		err := d.drawImageImmediately(&img.Image)
		if err.IsError() {
			img.Ctx.GetLogger().Errorf("Drawing to PixelDisplay failed: %v\n", err)
		}
		processingDuration := time.Now().Sub(start)
		img.Ctx.GetLogger().Debugf("Drawing took %s, delay from requesting to drawing was: %s", processingDuration, delay)
		if processingDuration < waitDuration {
			time.Sleep(waitDuration - processingDuration)
		}
	}
}
