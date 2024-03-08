//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookBluetooth struct {
	btw *FreepsBluetooth
}

var _ freepsgraph.FreepsGraphChangedHook = &HookBluetooth{}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookBluetooth) OnGraphChanged(ctx *base.Context, addedGraphName []string, removedGraphName []string) error {
	if h.btw == nil {
		return fmt.Errorf("Bluetooth watcher uninitialized")
	}
	h.btw.StopDiscovery(true)
	return nil
}
