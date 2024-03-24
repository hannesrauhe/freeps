//go:build nobluetooth || windows

package freepsbluetooth

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type Bluetooth struct {
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperatorWithConfig = &Bluetooth{}

// GetDefaultConfig returns a copy of the default config
func (bt *Bluetooth) GetDefaultConfig(fullName string) interface{} {
	return &BluetoothConfig{Enabled: false}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (bt *Bluetooth) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	return nil, fmt.Errorf("Bluetooth support not compiled in")
}
