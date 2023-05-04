package muteme

import (
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

// MuteMe implements the FreepsOperator interface to control the MuteMe button
type MuteMe struct {
	config MuteMeConfig
}

var _ base.FreepsOperatorWithConfig = &MuteMe{}

// GetConfig returns the config struct of the operator that is filled with the values from the config file
func (mm *MuteMe) GetConfig() interface{} {
	mm.config = DefaultConfig
	return &mm.config
}

// Init is called after the config is read and the operator is created
func (mm *MuteMe) Init(ctx *base.Context) error {
	var err error
	impl, err = newMuteMe(ctx, &mm.config)
	return err
}

// SetColorArgs are the arguments for the MuteMe-SetColor function
type SetColorArgs struct {
	Color string
}

var _ base.FreepsFunctionParameters = &SetColorArgs{}

// GetArgSuggestions returns suggestions for the color
func (mma *SetColorArgs) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
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

// SetColor sets the color of the MuteMe button
func (mm *MuteMe) SetColor(ctx *base.Context, input *base.OperatorIO, args SetColorArgs) *base.OperatorIO {
	if impl == nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Muteme not initialized")
	}
	if err := impl.SetColor(args.Color); err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Failed to set color: %v", err)
	}
	return base.MakePlainOutput(impl.GetColor())
}

// TurnOff turns off the MuteMe button
func (mm *MuteMe) TurnOff(ctx *base.Context) *base.OperatorIO {
	return mm.SetColor(ctx, nil, SetColorArgs{Color: "off"})
}

// Cycle cycles through the colors of the MuteMe button
func (mm *MuteMe) Cycle(ctx *base.Context) *base.OperatorIO {
	if impl == nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Muteme not initialized")
	}
	for c, b := range colors {
		if b != 0x00 && c != impl.GetColor() {
			return mm.SetColor(ctx, nil, SetColorArgs{Color: c})
		}
	}
	return base.MakePlainOutput(impl.GetColor())
}

// GetColor returns the current color of the MuteMe button
func (mm *MuteMe) GetColor() *base.OperatorIO {
	if impl == nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Muteme not initialized")
	}
	return base.MakePlainOutput(impl.GetColor())
}