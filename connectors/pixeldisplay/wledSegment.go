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

type WLEDSegment struct {
	ID    int         `json:"id"`
	I     [][3]uint32 `json:"i,omitempty"`
	Start *int        `json:"start,omitempty"`
	Stop  *int        `json:"stop,omitempty"`
	Len   *int        `json:"len,omitempty"`
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
	Seg WLEDSegment `json:"seg,omitempty"`
}

func newWLEDSegmentRoot(conf WLEDSegmentConfig) (*WLEDSegmentHolder, error) {
	return &WLEDSegmentHolder{conf: conf, actualLen: 0}, nil
}

func (h *WLEDSegmentHolder) SetImage(dst image.RGBA) ([]byte, error) {
	jsonob := WLEDRequest{}
	jsonob.Seg.ID = h.conf.SegID
	jsonob.Seg.I = make([][3]uint32, 0)
	for x := 0; x < h.conf.Width; x++ {
		for y := 0; y < h.conf.Height; y++ {
			j := y
			if x&1 != 0 {
				j = h.conf.Height - y - 1
			}
			r, g, b, _ := dst.At(x+h.conf.OffsetX, j+h.conf.OffsetY).RGBA()
			p := [3]uint32{r >> 8, g >> 8, b >> 8}
			jsonob.Seg.I = append(jsonob.Seg.I, p)
		}
	}
	if h.actualLen > 0 && len(jsonob.Seg.I) > h.actualLen {
		return nil, fmt.Errorf("Array of length %v for Segment %v longer than expected the %v pixels", len(jsonob.Seg.I), h.conf.SegID, h.actualLen)
	}
	return json.Marshal(jsonob)
}

func (h *WLEDSegmentHolder) SendToWLEDSegment(address string, dst image.RGBA) *base.OperatorIO {
	c := http.Client{}

	var b []byte
	var err error
	path := address + "/json"
	b, err = h.SetImage(dst)
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
