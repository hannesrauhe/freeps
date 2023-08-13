package pixeldisplay

import (
	"bytes"
	"encoding/json"
	"image"
	"io"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

type WLEDSegment struct {
	ID int         `json:"id"`
	I  [][3]uint32 `json:"i"`
}

type WLEDState struct {
	On bool `json:"on"`
	V  bool `json:"v"`
}

type WLEDSegmentConfig struct {
	Width   int
	Height  int
	SegID   int
	OffsetX int
	OffsetY int
}

type WLEDSegmentRoot struct {
	Seg  WLEDSegment `json:"seg,omitempty"`
	conf WLEDSegmentConfig
}

func newWLEDSegmentRoot(conf WLEDSegmentConfig) (*WLEDSegmentRoot, error) {
	return &WLEDSegmentRoot{conf: conf}, nil
}

func (root *WLEDSegmentRoot) SetImage(dst image.RGBA) ([]byte, error) {
	root.Seg.ID = root.conf.SegID
	root.Seg.I = make([][3]uint32, 0)
	for x := 0; x < root.conf.Width; x++ {
		for y := 0; y < root.conf.Height; y++ {
			j := y
			if x&1 != 0 {
				j = root.conf.Height - y - 1
			}
			r, g, b, _ := dst.At(x+root.conf.OffsetX, j+root.conf.OffsetY).RGBA()
			p := [3]uint32{r >> 8, g >> 8, b >> 8}
			root.Seg.I = append(root.Seg.I, p)
		}
	}
	return json.Marshal(root)
}

func (root *WLEDSegmentRoot) SendToWLEDSegment(address string, dst image.RGBA) *base.OperatorIO {
	c := http.Client{}

	var b []byte
	var err error
	path := address + "/json"
	b, err = root.SetImage(dst)
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
