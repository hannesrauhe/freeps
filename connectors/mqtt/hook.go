package mqtt

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookMQTT struct {
	impl *FreepsMqttImpl
}

var _ freepsgraph.FreepsHook = &HookMQTT{}

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
