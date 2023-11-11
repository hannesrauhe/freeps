package muteme

import (
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

// MuteMe implements the FreepsOperator interface to control the MuteMe button
type MuteMe struct {
	GE     *freepsgraph.GraphEngine
	config MuteMeConfig
	impl   *MuteMeImpl
}

var _ base.FreepsOperatorWithConfig = &MuteMe{}
var _ base.FreepsOperatorWithShutdown = &MuteMe{}

// GetDefaultConfig returns a copy of the default config
func (mm *MuteMe) GetDefaultConfig() interface{} {
	newConfig := DefaultConfig
	return &newConfig
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (mm *MuteMe) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	var err error
	newMM := MuteMe{config: *config.(*MuteMeConfig), GE: mm.GE}
	newMM.impl, err = newMuteMeImpl(ctx, &newMM.config)
	if err != nil {
		return nil, err
	}
	return &newMM, nil
}

// SetColorArgs are the arguments for the MuteMe-SetColor function
type SetColorArgs struct {
	Color string
}

// ColorSuggestions returns suggestions for the color
func (mma *SetColorArgs) ColorSuggestions() []string {
	r := make([]string, 0, len(colors))
	for c := range colors {
		r = append(r, c)
	}
	return r
}

// GetArgSuggestions returns suggestions for the color
func (mma *SetColorArgs) GetArgSuggestions(op base.FreepsOperator, fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// SetColor sets the color of the MuteMe button
func (mm *MuteMe) SetColor(ctx *base.Context, input *base.OperatorIO, args SetColorArgs) *base.OperatorIO {
	if err := mm.impl.SetColor(args.Color); err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Failed to set color: %v", err)
	}
	return base.MakePlainOutput(mm.impl.GetColor())
}

// TurnOff turns off the MuteMe button
func (mm *MuteMe) TurnOff(ctx *base.Context) *base.OperatorIO {
	return mm.SetColor(ctx, nil, SetColorArgs{Color: "off"})
}

// Cycle cycles through the colors of the MuteMe button
func (mm *MuteMe) Cycle(ctx *base.Context) *base.OperatorIO {
	for c, b := range colors {
		if b != 0x00 && c != mm.impl.GetColor() {
			return mm.SetColor(ctx, nil, SetColorArgs{Color: c})
		}
	}
	return base.MakePlainOutput(mm.impl.GetColor())
}

// GetColor returns the current color of the MuteMe button
func (mm *MuteMe) GetColor() *base.OperatorIO {
	return base.MakePlainOutput(mm.impl.GetColor())
}

// Shutdown the muteme listener
func (mm *MuteMe) Shutdown(ctx *base.Context) {
	mm.impl.Shutdown()
	mm.impl = nil
}

// StartListening starts the main loop of the muteme listener
func (mm *MuteMe) StartListening(ctx *base.Context) {
	go mm.impl.mainloop(mm.GE)
}
