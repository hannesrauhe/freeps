package opalert

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookAlert struct {
	impl *OpAlert
}

var _ freepsgraph.FreepsHook = &HookAlert{}

// GetName returns the name of the hook
func (h *HookAlert) GetName() string {
	return "alert"
}

// OnExecute does nothing
func (h *HookAlert) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnExecuteOperation does nothing
func (h *HookAlert) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	return nil
}

// OnExecutionError does nothing
func (h *HookAlert) OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *freepsgraph.GraphOperationDesc) error {
	return nil
}

// OnExecutionFinished does nothing
func (h *HookAlert) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookAlert) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookAlert) OnSystemAlert(ctx *base.Context, name string, severity int, err error) error {
	category := "system"
	errStr := err.Error()
	a := Alert{Name: name, Category: &category, Severity: &severity, Desc: &errStr}
	h.impl.SetAlert(ctx, base.MakeEmptyOutput(), a)
	return nil
}

// Shutdown gets called on graceful shutdown
func (h *HookAlert) Shutdown() error {
	return nil
}
