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

type ImagesWithMetadata struct {
	Animation []image.RGBA
	Created   time.Time
	Ctx       *base.Context
}

type WLEDMatrixDisplayConfig struct {
	Segments           []WLEDSegmentConfig
	Address            string
	MinDisplayDuration time.Duration
	ImageQueueSize     int
	MaxAnimationSize   int
}

type WLEDMatrixDisplay struct {
	segments            map[int]*WLEDSegmentHolder
	conf                *WLEDMatrixDisplayConfig
	height              int
	width               int
	backgroundLayerLock sync.Mutex
	backgroundLayer     map[string]image.RGBA
	imgChan             chan ImagesWithMetadata
	doneChan            chan struct{} // closed when the draw loop has stopped
	stopChan            chan struct{} // closed by Shutdown, makes the draw loop leave even with a full queue
	stopOnce            sync.Once
	color               color.Color
	bgColor             color.Color
}

// Timeout for WLED requests and the shutdown wait: an unreachable device must
// never block the shutdown of the daemon. Vars so tests can shorten them.
var (
	wledRequestTimeout   = 10 * time.Second
	shutdownWaitDuration = 15 * time.Second
)

// newWLEDHTTPClient returns an HTTP client that cannot block forever on an unreachable device.
func newWLEDHTTPClient() *http.Client {
	return &http.Client{Timeout: wledRequestTimeout}
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
	disp.imgChan = make(chan ImagesWithMetadata, cfg.ImageQueueSize)
	disp.doneChan = make(chan struct{})
	disp.stopChan = make(chan struct{})
	go disp.drawLoop(cfg.MinDisplayDuration)
	return disp, nil
}

func (d *WLEDMatrixDisplay) getState() (WLEDResponse, *base.OperatorIO) {
	var state WLEDResponse
	c := newWLEDHTTPClient()

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
	c := newWLEDHTTPClient()

	var b []byte
	var err error
	path := d.conf.Address + "/json/" + file
	b, err = cmd.GetBytes()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
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

// Shutdown stops the draw loop, waiting at most shutdownWaitDuration for it:
// a stuck or slow WLED device must not block the shutdown of the daemon.
func (d *WLEDMatrixDisplay) Shutdown(ctx *base.Context) {
	d.stopOnce.Do(func() {
		close(d.stopChan)
		close(d.imgChan)
	})
	ctx.GetLogger().Debugf("Waiting for drawing to pixeldisplay to finish\n")
	select {
	case <-d.doneChan:
	case <-time.After(shutdownWaitDuration):
		ctx.GetLogger().Warnf("Timeout while waiting for the pixeldisplay draw loop to stop, continuing shutdown without it")
	}
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

func (d *WLEDMatrixDisplay) DrawImage(ctx *base.Context, srcImg image.Image, returnPNG bool) *base.OperatorIO {
	if srcImg == nil {
		return base.MakeOutputError(http.StatusBadRequest, "no image to draw")
	}

	destBounds := image.Rect(0, 0, d.width, d.height)
	animation := []image.RGBA{}

	srcBounds := srcImg.Bounds()
	nextStartingPoint := destBounds.Min
	animationSize := srcBounds.Dx() - destBounds.Dx()
	// always draw the first picture, then draw as many as necessary but never more than configured
	for nextStartingPoint.X == 0 || (nextStartingPoint.X < animationSize && nextStartingPoint.X < d.conf.MaxAnimationSize) {
		converted := image.NewRGBA(destBounds)
		draw.Draw(converted, destBounds, srcImg, nextStartingPoint, draw.Src)
		animation = append(animation, *converted)
		nextStartingPoint.X++
	}

	select {
	case d.imgChan <- ImagesWithMetadata{Animation: animation, Created: time.Now(), Ctx: ctx}:
	default:
		return base.MakeOutputError(http.StatusTooManyRequests, "Too many images in queue")
	}

	if !returnPNG || 0 == len(animation) {
		return base.MakeEmptyOutput()
	}
	var bout []byte
	contentType := "image/png"
	writer := bytes.NewBuffer(bout)
	if err := png.Encode(writer, &animation[0]); err != nil {
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
	return image.Point{X: d.width + d.conf.MaxAnimationSize, Y: d.height}
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

// drawLoop draws images from the queue to the display, sleeping waitDuration between pictures.
// It leaves when the queue is drained or stopChan is closed, and closes doneChan on exit.
func (d *WLEDMatrixDisplay) drawLoop(waitDuration time.Duration) {
	timeoutDuration := 2 * time.Minute
	defer close(d.doneChan)
	for {
		var animationWithMetadata ImagesWithMetadata
		select {
		case <-d.stopChan:
			return
		case a, ok := <-d.imgChan:
			if !ok {
				return
			}
			animationWithMetadata = a
		}

		for numPic, singleImage := range animationWithMetadata.Animation {
			select {
			case <-d.stopChan:
				return
			default:
			}
			delay := time.Now().Sub(animationWithMetadata.Created)
			if delay > timeoutDuration {
				animationWithMetadata.Ctx.GetLogger().Errorf("Timeout when drawing to Pixeldisplay, delay is: %s, skipping next picture/rest of animation", delay)
				break
			}

			start := time.Now()
			err := d.drawImageImmediately(&singleImage)
			if err.IsError() {
				animationWithMetadata.Ctx.GetLogger().Errorf("Drawing to PixelDisplay failed: %v\n", err)
			}
			processingDuration := time.Now().Sub(start)
			animationWithMetadata.Ctx.GetLogger().Debugf("Drawing took %s, delay from requesting first picture of Animation to drawing %v. picture was: %s", processingDuration, numPic, delay)
			if processingDuration < waitDuration {
				time.Sleep(waitDuration - processingDuration)
			}
		}
	}
}
