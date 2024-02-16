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

var _ freepsgraph.FreepsHook = HookBluetooth{}

// GetName returns the name of the hook
func (h HookBluetooth) GetName() string {
	return "bluetooth"
}

// OnExecute does nothing
func (h HookBluetooth) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnExecuteOperation does nothing
func (h HookBluetooth) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	return nil
}

// OnExecutionError does nothing
func (h HookBluetooth) OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *freepsgraph.GraphOperationDesc) error {
	return nil
}

// OnExecutionFinished does nothing
func (h HookBluetooth) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h HookBluetooth) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	if h.btw == nil {
		return fmt.Errorf("Bluetooth watcher uninitialized")
	}
	h.btw.StopDiscovery(true)
	return nil
}

func (h HookBluetooth) OnSystemAlert(ctx *base.Context, name string, severity int, err error) error {
	return nil
}

// Shutdown gets called on graceful shutdown
func (h HookBluetooth) Shutdown() error {
	return nil
}
