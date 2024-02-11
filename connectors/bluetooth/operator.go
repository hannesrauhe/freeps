//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"math"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/muka/go-bluetooth/api"
)

// Bluetooth is the operator that provides bluetooth functionality and implements the FreepsGenericOperator interface
type Bluetooth struct {
	GE     *freepsgraph.GraphEngine
	config *BluetoothConfig
	btw    *FreepsBluetooth
}

var _ base.FreepsOperatorWithConfig = &Bluetooth{}
var _ base.FreepsOperatorWithShutdown = &Bluetooth{}
var _ base.FreepsOperatorWithHook = &Bluetooth{}

// GetDefaultConfig returns a copy of the default config
func (bt *Bluetooth) GetDefaultConfig() interface{} {
	cfg := defaultBluetoothConfig
	return &cfg
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (bt *Bluetooth) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	btc := config.(*BluetoothConfig)
	btlogger := ctx.GetLogger().WithField("component", "bluetooth")
	newBT := &Bluetooth{GE: bt.GE, config: btc,
		btw: &FreepsBluetooth{
			config: btc, log: btlogger, shuttingDown: false, ge: bt.GE, monitors: &monitors{watchers: map[string]deviceEntry{}}}}

	return newBT, nil
}

// Shutdown shuts down the operator
func (bt *Bluetooth) Shutdown(ctx *base.Context) {
	if bt.btw != nil {
		bt.btw.Shutdown()
	}
}

// StartListening starts the operator
func (bt *Bluetooth) StartListening(ctx *base.Context) {
	err := bt.btw.StartDiscovery()
	if err != nil {
		ctx.GetLogger().WithError(err).Error("Error starting bluetooth discovery")
		bt.btw = nil
		api.Exit()
	}
}

// GetHook returns the hook for this operator
func (bt *Bluetooth) GetHook() interface{} {
	if bt.btw == nil {
		return nil
	}
	return HookBluetooth{btw: bt.btw}
}

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

	res := store.GetNamespaceNoError(bt.config.DiscoveredNamespace).GetSearchResultWithMetadata("", "", "", 0, maxAge)
	m2 := store.GetNamespaceNoError(bt.config.KnownNamespace).GetSearchResultWithMetadata("", "", "", 0, maxAge)

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
	bt.btw.StopDiscovery(true)
	return base.MakeEmptyOutput()
}
