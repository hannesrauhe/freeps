package freepsflow

import (
	"reflect"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type FlowEngineHook interface {
	GetName() string
}

type FreepsExecutionHook interface {
	OnExecute(ctx *base.Context, flowName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
	OnExecuteOperation(ctx *base.Context, input *base.OperatorIO, opOutput *base.OperatorIO, flowName string, od *FlowOperationDesc) error
	OnExecutionFinished(ctx *base.Context, flowName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
}

type FreepsFlowChangedHook interface {
	OnFlowChanged(ctx *base.Context, addedFlowName []string, removedFlowName []string) error
}

type FreepsAlertHook interface {
	OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error, expiresIn *time.Duration) error
	OnResetSystemAlert(ctx *base.Context, name string, category string) error
}

type FreepsHookWrapper struct {
	hookImpl interface{}
}

var _ FlowEngineHook = &FreepsHookWrapper{}
var _ FreepsExecutionHook = &FreepsHookWrapper{}
var _ FreepsFlowChangedHook = &FreepsHookWrapper{}
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

func (h *FreepsHookWrapper) OnExecute(ctx *base.Context, flowName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	i, ok := h.hookImpl.(FreepsExecutionHook)
	if ok {
		return i.OnExecute(ctx, flowName, mainArgs, mainInput)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecuteOperation(ctx *base.Context, input *base.OperatorIO, opOutput *base.OperatorIO, flowName string, od *FlowOperationDesc) error {
	i, ok := h.hookImpl.(FreepsExecutionHook)
	if ok {
		return i.OnExecuteOperation(ctx, input, opOutput, flowName, od)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecutionFinished(ctx *base.Context, flowName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	i, ok := h.hookImpl.(FreepsExecutionHook)
	if ok {
		return i.OnExecutionFinished(ctx, flowName, mainArgs, mainInput)
	}
	return nil
}

func (h *FreepsHookWrapper) OnFlowChanged(ctx *base.Context, addedFlowName []string, removedFlowName []string) error {
	i, ok := h.hookImpl.(FreepsFlowChangedHook)
	if ok {
		return i.OnFlowChanged(ctx, addedFlowName, removedFlowName)
	}
	return nil
}

func (h *FreepsHookWrapper) OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error, expiresIn *time.Duration) error {
	i, ok := h.hookImpl.(FreepsAlertHook)
	if ok {
		i.OnSystemAlert(ctx, name, category, severity, err, expiresIn)
	}
	return nil
}

func (h *FreepsHookWrapper) OnResetSystemAlert(ctx *base.Context, name string, category string) error {
	i, ok := h.hookImpl.(FreepsAlertHook)
	if ok {
		i.OnResetSystemAlert(ctx, name, category)
	}
	return nil
}
