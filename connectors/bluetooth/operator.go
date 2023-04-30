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
func (bt *Bluetooth) GetPresentDevices(ctx *base.Context, input *base.OperatorIO, gpd GetPresentDevicesParams) *base.OperatorIO {
	store := freepsstore.GetGlobalStore()

	res := store.GetNamespace(bt.config.DiscoveredNamespace).GetSearchResultWithMetadata("", "", "", 0, gpd.MaximumAge)
	m2 := store.GetNamespace(bt.config.KnownNamespace).GetSearchResultWithMetadata("", "", "", 0, gpd.MaximumAge)
	m3 := store.GetNamespace(bt.config.MonitorsNamespace).GetSearchResultWithMetadata("", "", "", 0, gpd.MaximumAge)

	// merge the results, on conflict the newer entry wins
	for k, v2 := range m2 {
		v1, ok := res[k]
		if !ok {
			res[k] = v2
		} else if v2.GetTimestamp().After(v1.GetTimestamp()) {
			res[k] = v2
		}
	}
	for k, v3 := range m3 {
		v1, ok := res[k]
		if !ok {
			res[k] = v3
		} else if v3.GetTimestamp().After(v1.GetTimestamp()) {
			res[k] = v3
		}
	}

	// return the keys
	return base.MakeObjectOutput(res)
}

// GetArgSuggestions returns common durations for the maximumage parameter
func (gpd *GetPresentDevicesParams) GetArgSuggestions(fn string, argName string, otherArgs map[string]string) map[string]string {
	if argName == "maximumage" {
		return map[string]string{"1m": "1 min", "10m": "10 min", "1h": "1 hour", "1d": "1 day"}
	}
	return map[string]string{}
}
