//go:build nomuteme || windows

package muteme

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

// MuteMe implements the FreepsOperator interface to control the MuteMe button
type MuteMe struct {
	GE     *freepsgraph.GraphEngine
	config MuteMeConfig
}

type MuteMeConfig struct {
	Enabled bool // if false, the muteme button will be ignored
}

var _ base.FreepsOperatorWithConfig = &MuteMe{}

// GetDefaultConfig returns a config with the button disabled
func (mm *MuteMe) GetDefaultConfig() interface{} {
	return &MuteMeConfig{
		Enabled: false,
	}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (mm *MuteMe) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	newMM := MuteMe{config: *config.(*MuteMeConfig), GE: mm.GE}
	return &newMM, nil
}


