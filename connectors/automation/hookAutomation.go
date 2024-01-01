package automation

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookAutomation struct {
	oa *OpAutomation
}

var _ freepsgraph.FreepsHook = HookAutomation{}

// GetName returns the name of the hook
func (h HookAutomation) GetName() string {
	return "automation"
}

// OnExecute does nothing
func (h HookAutomation) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnExecuteOperation does nothing
func (h HookAutomation) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	return nil
}

// OnExecutionError does nothing
func (h HookAutomation) OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *freepsgraph.GraphOperationDesc) error {
	return nil
}

// OnExecutionFinished does nothing
func (h HookAutomation) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	return nil
}

// OnGraphChanged checks if subscriptions need to be changed
func (h HookAutomation) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	if h.oa == nil {
		return fmt.Errorf("Automation operator uninitialized")
	}
	h.oa.buildRuleAndTriggerMap()
	return nil
}

// Shutdown gets called on graceful shutdown
func (h HookAutomation) Shutdown() error {
	return nil
}
