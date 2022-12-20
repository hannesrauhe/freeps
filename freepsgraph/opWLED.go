package freepsgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hannesrauhe/freeps/utils"
)

type OpWLED struct {
}

var _ FreepsOperator = &OpWLED{}

// GetName returns the name of the operator
func (o *OpWLED) GetName() string {
	return "wled"
}

func (o *OpWLED) Execute(ctx *utils.Context, function string, vars map[string]string, mainInput *OperatorIO) *OperatorIO {
	c := http.Client{}

	var resp *http.Response
	var err error
	vars["address"] = "http://wled-matrix.fritz.box"
	switch function {
	case "setPixel":
		var b []byte
		if vars["body"] == "" {
			w := WLEDConverter{r: WLEDRoot{}, x: 16, y: 8}
			w.SetPixel(0, 0, 255, 0, 0)
			w.SetPixel(1, 1, 255, 0, 0)
			w.SetPixel(2, 2, 255, 0, 0)
			w.SetPixel(4, 4, 255, 0, 0)
			w.SetPixel(15, 7, 255, 0, 0)
			// {"seg":{"i":[[0,0,0],[255,0,0],[255,0,0],[0,255,0],[0,255,255]]}}
			// {"seg":{"i":[[255,0,0], [0,255,0], [0,0,255]]}}

			b, err = w.GetJSON()
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, err.Error())
			}
		} else {
			b, err = mainInput.GetBytes()
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, err.Error())
			}
		}

		breader := bytes.NewReader(b)
		resp, err = c.Post(vars["address"]+"/json", "encoding/json", breader)
	default:
		return MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if err != nil {
		return MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return &OperatorIO{HTTPCode: resp.StatusCode, Output: b, OutputType: Byte, ContentType: resp.Header.Get("Content-Type")}
}

func (o *OpWLED) GetFunctions() []string {
	return []string{"test"}
}

func (o *OpWLED) GetPossibleArgs(fn string) []string {
	return []string{"address"}
}

func (o *OpWLED) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpWLED) Shutdown(ctx *utils.Context) {
}

type WLEDSegment struct {
	I [][3]int `json:"i"`
}

type WLEDRoot struct {
	Seg WLEDSegment `json:"seg"`
}

type WLEDConverter struct {
	r        WLEDRoot
	x        int
	y        int
	pixelmap map[int]map[int][3]int
}

func (w *WLEDConverter) AppendIndividualPixel(r int, g int, b int) {
	if w.r.Seg.I == nil {
		w.r.Seg.I = make([][3]int, 0)
	}
	w.r.Seg.I = append(w.r.Seg.I, [3]int{r, g, b})
}

func (w *WLEDConverter) SetPixel(x, y, r, g, b int) error {
	if x >= w.x {
		return fmt.Errorf("x dimension out of bounds")
	}
	if y >= w.y {
		return fmt.Errorf("y dimension out of bounds")
	}
	if w.pixelmap == nil {
		w.pixelmap = make(map[int]map[int][3]int)
	}
	if w.pixelmap[x] == nil {
		w.pixelmap[x] = make(map[int][3]int)
	}
	w.pixelmap[x][y] = [3]int{r, g, b}
	return nil
}

func (w *WLEDConverter) GetJSON() ([]byte, error) {
	if w.r.Seg.I == nil {
		w.r.Seg.I = make([][3]int, 0)
	}
	for x := 0; x < w.x; x++ {
		for y := 0; y < w.y; y++ {
			j := y
			if x&1 != 0 {
				j = w.y - y - 1
			}
			p := [3]int{0, 0, 0}
			if s, ok := w.pixelmap[x][j]; ok {
				p = s
			}
			w.r.Seg.I = append(w.r.Seg.I, p)
		}
	}
	return json.Marshal(w.r)
}
