package freepsbluetooth

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type HookBluetooth struct {
}

var _ freepsgraph.FreepsHook = HookBluetooth{}

// NewMQTTHook creates a Hook to subscribe to topics when graphs change
func NewMQTTHook(cr *utils.ConfigReader) (HookBluetooth, error) {
	return HookBluetooth{}, nil
}

// GetName returns the name of the hook
func (h HookBluetooth) GetName() string {
	return "mqtt"
}

// OnExecute does nothing
func (h HookBluetooth) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	return nil
}

// OnExecutionFinished does nothing
func (h HookBluetooth) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h HookBluetooth) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	if btwatcher == nil {
		return fmt.Errorf("Bluetooth watcher uninitilaized")
	}
	return btwatcher.StartSupscription()
}

// Shutdown gets called on graceful shutdown
func (h HookBluetooth) Shutdown() error {
	return nil
}
