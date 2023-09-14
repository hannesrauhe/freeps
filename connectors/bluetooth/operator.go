//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"math"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
)

// Bluetooth is the operator that provides bluetooth functionality and implements the FreepsGenericOperator interface
type Bluetooth struct {
	config BluetoothConfig
}

// GetDefaultConfig returns a copy of the default config
func (bt *Bluetooth) GetDefaultConfig() interface{} {
	return &defaultBluetoothConfig
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (bt *Bluetooth) InitCopyOfOperator(config interface{}, ctx *base.Context) (base.FreepsOperatorWithConfig, error) {
	return bt, nil
}

var _ base.FreepsOperatorWithConfig = &Bluetooth{}

// GetPresentDevicesParams are the parameters for the GetPresentDevices Function
type GetPresentDevicesParams struct {
	MaxAge *time.Duration
}

// GetPresentDevices is the function that returns the present devices
func (bt *Bluetooth) GetPresentDevices(ctx *base.Context, input *base.OperatorIO, gpd GetPresentDevicesParams) *base.OperatorIO {
	store := freepsstore.GetGlobalStore()
	maxAge := time.Duration(math.MaxInt64)
	if gpd.MaxAge != nil {
		maxAge = *gpd.MaxAge
	}

	res := store.GetNamespace(bt.config.DiscoveredNamespace).GetSearchResultWithMetadata("", "", "", 0, maxAge)
	m2 := store.GetNamespace(bt.config.KnownNamespace).GetSearchResultWithMetadata("", "", "", 0, maxAge)

	// merge the results, on conflict the newer entry wins
	for k, v2 := range m2 {
		v1, ok := res[k]
		if !ok {
			res[k] = v2
		} else if v2.GetTimestamp().After(v1.GetTimestamp()) {
			res[k] = v2
		}
	}

	return base.MakeObjectOutput(res)
}

// RestartDiscovery triggers the Discovery process immediately
func (bt *Bluetooth) RestartDiscovery(ctx *base.Context) *base.OperatorIO {
	btwatcher.StopDiscovery(true)
	return base.MakeEmptyOutput()
}
