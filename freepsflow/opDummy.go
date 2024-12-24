package freepsflow

import (
	"github.com/hannesrauhe/freeps/base"
)

type DummyOperator struct {
	GE     *FlowEngine
	config DummyConfig
}

type DummyConfig struct {
	Enabled bool
}

var _ base.FreepsOperatorWithConfig = &DummyOperator{}

func (d *DummyOperator) GetDefaultConfig() interface{} {
	return &DummyConfig{
		Enabled: false,
	}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (d *DummyOperator) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	newMM := DummyOperator{config: *config.(*DummyConfig), GE: d.GE}
	return &newMM, nil
}
