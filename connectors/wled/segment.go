package wled

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"

	"github.com/hannesrauhe/freeps/freepsgraph"
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

var availableCommands = map[string]WLEDState{
	"on":  {On: true, V: true},
	"off": {On: false, V: true},
}

func (root *WLEDRoot) SetCmd(cmd string) ([]byte, error) {
	state, ok := availableCommands[cmd]
	if !ok {
		return nil, fmt.Errorf("cmd %s not found", cmd)
	}

	return json.Marshal(state)
}

func (root *WLEDRoot) SendToWLED(dst *image.RGBA) *freepsgraph.OperatorIO {
	c := http.Client{}

	b, err := root.SetImage(dst)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err := c.Post(root.conf.Address+"/json", "application/json", breader)

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}

	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &freepsgraph.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: freepsgraph.Byte, ContentType: resp.Header.Get("Content-Type")}
}

func (root *WLEDRoot) WLEDCommand(cmd string) *freepsgraph.OperatorIO {
	c := http.Client{}

	b, err := root.SetCmd(cmd)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	breader := bytes.NewReader(b)
	resp, err := c.Post(root.conf.Address+"/json/state", "application/json", breader)

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}

	defer resp.Body.Close()
	bout, err := io.ReadAll(resp.Body)
	return &freepsgraph.OperatorIO{HTTPCode: resp.StatusCode, Output: bout, OutputType: freepsgraph.Byte, ContentType: resp.Header.Get("Content-Type")}
}
