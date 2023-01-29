package mqtt

import (
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type HookMQTT struct {
}

var _ freepsgraph.FreepsHook = &HookMQTT{}

// NewMQTTHook creates a Hook to subscribe to topics when graphs change
func NewMQTTHook(cr *utils.ConfigReader) (*HookMQTT, error) {
	return &HookMQTT{}, nil
}

// GetName returns the name of the hook
func (h *HookMQTT) GetName() string {
	return "store"
}

// OnExecute does nothing
func (h *HookMQTT) OnExecute(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	return nil
}

// OnExecutionFinished does nothing
func (h *HookMQTT) OnExecutionFinished(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookMQTT) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	return GetInstance().SubscribeToTags()
}

// Shutdown gets called on graceful shutdown
func (h *HookMQTT) Shutdown() error {
	return nil
}
