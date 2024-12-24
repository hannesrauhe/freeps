//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
)

type HookBluetooth struct {
	btw *FreepsBluetooth
}

var _ freepsflow.FreepsFlowChangedHook = &HookBluetooth{}

// OnFlowChanged checks if subscriptions need to be changed
func (h *HookBluetooth) OnFlowChanged(ctx *base.Context, addedFlowName []string, removedFlowName []string) error {
	if h.btw == nil {
		return fmt.Errorf("Bluetooth watcher uninitialized")
	}
	h.btw.StopDiscovery(true)
	return nil
}
