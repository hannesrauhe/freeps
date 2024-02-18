package freepsgraph

import (
	"reflect"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type GraphEngineHook interface {
	GetName() string
}

type FreepsHook interface {
	OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
	OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error
	OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *GraphOperationDesc) error
	OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
	OnGraphChanged(addedGraphName []string, removedGraphName []string) error
}

type FreepsAlertHook interface {
	OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error) error
}

type FreepsHookWrapper struct {
	hookImpl interface{}
}

var _ GraphEngineHook = &FreepsHookWrapper{}
var _ FreepsHook = &FreepsHookWrapper{}
var _ FreepsAlertHook = &FreepsHookWrapper{}

func NewFreepsHookWrapper(hookImpl interface{}) *FreepsHookWrapper {
	return &FreepsHookWrapper{hookImpl: hookImpl}
}

func (h *FreepsHookWrapper) GetName() string {
	t := reflect.TypeOf(h.hookImpl)
	fullName := t.Elem().Name()
	if utils.StringStartsWith(fullName, "Hook") {
		return fullName[4:]
	}
	return fullName
}

func (h *FreepsHookWrapper) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	i, ok := h.hookImpl.(FreepsHook)
	if ok {
		return i.OnExecute(ctx, graphName, mainArgs, mainInput)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	i, ok := h.hookImpl.(FreepsHook)
	if ok {
		return i.OnExecuteOperation(ctx, operationIndexInContext)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *GraphOperationDesc) error {
	i, ok := h.hookImpl.(FreepsHook)
	if ok {
		return i.OnExecutionError(ctx, input, err, graphName, od)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	i, ok := h.hookImpl.(FreepsHook)
	if ok {
		return i.OnExecutionFinished(ctx, graphName, mainArgs, mainInput)
	}
	return nil
}

func (h *FreepsHookWrapper) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	i, ok := h.hookImpl.(FreepsHook)
	if ok {
		return i.OnGraphChanged(addedGraphName, removedGraphName)
	}
	return nil
}

func (h *FreepsHookWrapper) OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error) error {
	i, ok := h.hookImpl.(FreepsAlertHook)
	if ok {
		i.OnSystemAlert(ctx, name, category, severity, err)
	}
	return nil
}
