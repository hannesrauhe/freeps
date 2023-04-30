//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
)

// Bluetooth is the operator that provides bluetooth functionality and implements the FreepsGenericOperator interface
type Bluetooth struct {
	config BluetoothConfig
}

// GetConfig returns the config for the bluetooth operator
func (*Bluetooth) GetConfig() interface{} {
	config := defaultBluetoothConfig
	return &config
}

// Init initializes the bluetooth operator
func (bt *Bluetooth) Init(ctx *base.Context) error {
	return nil
}

var _ base.FreepsOperatorWithConfig = &Bluetooth{}

// GetPresentDevicesParams are the parameters for the GetPresentDevices Function
type GetPresentDevicesParams struct {
	MaximumAge time.Duration
}

// GetPresentDevices is the function that returns the present devices
func (bt *Bluetooth) GetPresentDevices(ctx *base.Context, input *base.OperatorIO, gpd GetPresentDevicesParams, otherArgs map[string]string) *base.OperatorIO {
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
func (gpd *GetPresentDevicesParams) GetArgSuggestions(fn string, argName string) map[string]string {
	if argName == "maximumage" {
		return map[string]string{"1m": "1 min", "10m": "10 min", "1h": "1 hour", "1d": "1 day"}
	}
	return map[string]string{}
}
