package muteme

import (
	"net/http"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpMuteMe struct {
	mm *MuteMe
}

var _ freepsgraph.FreepsOperator = &OpMuteMe{}

func NewMuteMeOp(mm *MuteMe) *OpMuteMe {
	fmqtt := &OpMuteMe{mm: mm}
	return fmqtt
}

// GetName returns the name of the operator
func (o *OpMuteMe) GetName() string {
	return "muteme"
}

func (o *OpMuteMe) Execute(ctx *utils.Context, fn string, args map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	switch fn {
	case "setColor":
		return o.mm.SetColor(args["color"])
	case "turnOff":
		return o.mm.SetColor("off")
	case "cycle":
		for c, b := range colors {
			if b != 0x00 && c != o.mm.GetColor() {
				return o.mm.SetColor(c)
			}
		}
	case "getColor":
		return freepsgraph.MakePlainOutput(o.mm.GetColor())
	default:
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Unknown function "+fn)
	}

	return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Unexpected error")
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
