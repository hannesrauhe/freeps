//go:build noinflux

package freepsflux

import (
	"github.com/hannesrauhe/freeps/base"
)

type OperatorFlux struct {
}

var _ base.FreepsOperatorWithConfig = &OperatorFlux{}

func (o *OperatorFlux) Init(ctx *base.Context) error {
	return nil
}

func (o *OperatorFlux) ResetConfigToDefault() interface{} {
	return &FreepsFluxConfig{Enabled: false}
}
