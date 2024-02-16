package mqtt

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookMQTT struct {
	impl *FreepsMqttImpl
}

var _ freepsgraph.FreepsHook = &HookMQTT{}

// GetName returns the name of the hook
func (h *HookMQTT) GetName() string {
	return "mqtt"
}

// OnExecute does nothing
func (h *HookMQTT) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnExecuteOperation does nothing
func (h *HookMQTT) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	return nil
}

// OnExecutionError does nothing
func (h *HookMQTT) OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *freepsgraph.GraphOperationDesc) error {
	return nil
}

// OnExecutionFinished does nothing
func (h *HookMQTT) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookMQTT) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	return h.impl.startTagSubscriptions()
}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookMQTT) OnSystemAlert(ctx *base.Context, name string, severity int, err error) error {
	return nil
}

// Shutdown gets called on graceful shutdown
func (h *HookMQTT) Shutdown() error {
	return nil
}
