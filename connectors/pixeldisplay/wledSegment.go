package pixeldisplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

type WLEDSegmentReqeust struct {
	ID int         `json:"id"`
	I  [][3]uint32 `json:"i,omitempty"`
}

type WLEDSegmentConfig struct {
	Width   int
	Height  int
	SegID   int
	OffsetX int
	OffsetY int
}

type WLEDSegmentHolder struct {
	conf      WLEDSegmentConfig
	actualLen int
}

type WLEDRequest struct {
	Seg WLEDSegmentReqeust `json:"seg,omitempty"`
}

func newWLEDSegmentHolder(conf WLEDSegmentConfig) (*WLEDSegmentHolder, error) {

	return &WLEDSegmentHolder{conf: conf, actualLen: 0}, nil
}

func (h *WLEDSegmentHolder) convertImageToWLEDRequest(dst image.RGBA, jsonb WLEDRequest) {
	outputIndex := 0
	for x := 0; x < h.conf.Width; x++ {
		for y := 0; y < h.conf.Height; y++ {
			j := y
			if x&1 != 0 {
				j = h.conf.Height - y - 1
			}
			r, g, b, a := dst.At(x+h.conf.OffsetX, j+h.conf.OffsetY).RGBA()
			if a != 0 {
				p := [3]uint32{r >> 8, g >> 8, b >> 8}
				jsonb.Seg.I[outputIndex] = p
			}
			outputIndex++
		}
	}
}

func (h *WLEDSegmentHolder) SendToWLEDSegment(address string, dst image.RGBA, background *image.RGBA) *base.OperatorIO {
	// TODO(HR): assertion that should probably go somewhere else
	if h.actualLen > 0 && h.conf.Width*h.conf.Height > h.actualLen {
		return base.MakeOutputError(http.StatusBadRequest, "Array of length %v for Segment %v longer than expected the %v pixels", h.conf.Width*h.conf.Height, h.conf.SegID, h.actualLen)
	}

	c := http.Client{}

	path := address + "/json"

	jsonob := WLEDRequest{}
	jsonob.Seg.ID = h.conf.SegID
	jsonob.Seg.I = make([][3]uint32, h.conf.Width*h.conf.Height)
	if background != nil {
		h.convertImageToWLEDRequest(*background, jsonob)
	}
	h.convertImageToWLEDRequest(dst, jsonob)
	b, err := json.Marshal(jsonob)
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
	if err != nil {
		// TODO(HR): error handling
		fmt.Printf("\n%v\n", err)
	}
	return &base.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: base.Byte, ContentType: resp.Header.Get("Content-Type")}
}
