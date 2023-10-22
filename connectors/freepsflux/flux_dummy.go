//go:build noinflux

package freepsflux

import (
	"github.com/hannesrauhe/freeps/base"
)

type OperatorFlux struct {
}

var _ base.FreepsOperatorWithConfig = &OperatorFlux{}

// GetDefaultConfig returns a copy of the default config
func (o *OperatorFlux) GetDefaultConfig() interface{} {
	return &FreepsFluxConfig{Enabled: false}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (o *OperatorFlux) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	return nil, nil
}
