package usb

import (
	"net/http"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpMuteMe struct {
}

var _ freepsgraph.FreepsOperator = &OpMuteMe{}

func NewMuteMeOp(cr *utils.ConfigReader) *OpMuteMe {
	fmqtt := &OpMuteMe{}
	return fmqtt
}

// GetName returns the name of the operator
func (o *OpMuteMe) GetName() string {
	return "muteme"
}

func (o *OpMuteMe) Execute(ctx *utils.Context, fn string, args map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	switch fn {
	case "setColor":
		return GetInstance().SetColor(args["color"])
	case "turnOff":
		return GetInstance().SetColor("off")
	case "cycle":
		for c, b := range colors {
			if b != 0x00 && c != GetInstance().GetColor() {
				return GetInstance().SetColor(c)
			}
		}
	case "getColor":
		return freepsgraph.MakePlainOutput(GetInstance().GetColor())
	}
	return freepsgraph.MakeOutputError(http.StatusBadRequest, "Unknown function "+fn)
}

func (o *OpMuteMe) GetFunctions() []string {
	return []string{"setColor", "cycle", "getColor", "turnOff"}
}

func (o *OpMuteMe) GetPossibleArgs(fn string) []string {
	return []string{"color"}
}

func (o *OpMuteMe) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "color":
		r := map[string]string{}
		for c, _ := range colors {
			r[c] = c
		}
		return r
	}

	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpMuteMe) Shutdown(ctx *utils.Context) {
}
