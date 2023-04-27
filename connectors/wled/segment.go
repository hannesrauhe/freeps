package wled

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

type WLEDRoot struct {
	Seg     WLEDSegment `json:"seg,omitempty"`
	conf    WLEDConfig
	offsetX int
	offsetY int
}

func newWLEDRoot(conf WLEDConfig, offsetX, offsetY int) (*WLEDRoot, error) {
	err := conf.Validate(true)
	if err != nil {
		return nil, err
	}
	return &WLEDRoot{conf: conf, offsetX: offsetX, offsetY: offsetY}, nil
}

func (root *WLEDRoot) Width() int {
	return root.conf.Width
}

func (root *WLEDRoot) Height() int {
	return root.conf.Height
}

func (root *WLEDRoot) SetImage(dst *image.RGBA) ([]byte, error) {
	root.Seg.ID = root.conf.SegID
	root.Seg.I = make([][3]uint32, 0)
	for x := 0; x < root.Width(); x++ {
		for y := 0; y < root.Height(); y++ {
			j := y
			if x&1 != 0 {
				j = root.Height() - y - 1
			}
			r, g, b, _ := dst.At(x+root.offsetX, j+root.offsetY).RGBA()
			p := [3]uint32{r >> 8, g >> 8, b >> 8}
			root.Seg.I = append(root.Seg.I, p)
		}
	}
	return json.Marshal(root)
}

func (root *WLEDRoot) SendToWLED(cmd *base.OperatorIO, dst *image.RGBA) *base.OperatorIO {
	c := http.Client{}

	var b []byte
	var err error
	path := root.conf.Address + "/json"
	if dst != nil {
		b, err = root.SetImage(dst)
	} else {
		path += "/state"
		b, err = cmd.GetBytes()
	}
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
