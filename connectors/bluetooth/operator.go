//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
)

// Bluetooth is the operator that provides bluetooth functionality and implements the FreepsGenericOperator interface
type Bluetooth struct {
}

var _ base.FreepsOperator = &Bluetooth{}

// GetPresentDevices returns a list of present devices
func (bt *Bluetooth) GetPresentDevices() *GetPresentDevices {
	return &GetPresentDevices{}
}

// GetPresentDevices implements the FreepsGenericFunction interface
type GetPresentDevices struct {
	MaximumAge time.Duration
}

var _ base.FreepsFunction = &GetPresentDevices{}

// Run returns a list of present devices
func (gpd *GetPresentDevices) Run(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	// get the global store
	store := freepsstore.GetGlobalStore()

	// get the keys of the _bluetooth_discovered_devices namespace
	discoveredKeys := store.GetNamespace("_bluetooth_discovered_devices").GetKeys()

	// get the keys of the _bluetooth_known_devices namespace
	knownKeys := store.GetNamespace("_bluetooth_known_devices").GetKeys()

	// merge the keys
	keys := append(discoveredKeys, knownKeys...)

	// return the keys
	return base.MakeObjectOutput(keys)
}

// GetArgSuggestions implements the FreepsGenericFunction interface and returns common durations
func (gpd *GetPresentDevices) GetArgSuggestions(argName string) map[string]string {
	if argName == "maximumage" {
		return map[string]string{"1m": "1 min", "10m": "10 min", "1h": "1 hour", "1d": "1 day"}
	}
	return map[string]string{}
}
