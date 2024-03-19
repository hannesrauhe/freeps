package freepsgraph

import (
	"reflect"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type GraphEngineHook interface {
	GetName() string
}

type FreepsExecutionHook interface {
	OnExecuteOld(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
	OnExecuteOperation(ctx *base.Context, input *base.OperatorIO, opOutput *base.OperatorIO, graphName string, od *GraphOperationDesc) error
	OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
}

type FreepsGraphChangedHook interface {
	OnGraphChanged(ctx *base.Context, addedGraphName []string, removedGraphName []string) error
}

type FreepsAlertHook interface {
	OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error, expiresIn *time.Duration) error
	OnResetSystemAlert(ctx *base.Context, name string, category string) error
}

type FreepsHookWrapper struct {
	hookImpl interface{}
}

var _ GraphEngineHook = &FreepsHookWrapper{}
var _ FreepsExecutionHook = &FreepsHookWrapper{}
var _ FreepsGraphChangedHook = &FreepsHookWrapper{}
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

func (h *FreepsHookWrapper) OnExecuteOld(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	i, ok := h.hookImpl.(FreepsExecutionHook)
	if ok {
		return i.OnExecuteOld(ctx, graphName, mainArgs, mainInput)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecuteOperation(ctx *base.Context, input *base.OperatorIO, opOutput *base.OperatorIO, graphName string, od *GraphOperationDesc) error {
	i, ok := h.hookImpl.(FreepsExecutionHook)
	if ok {
		return i.OnExecuteOperation(ctx, input, opOutput, graphName, od)
	}
	return nil
}

func (h *FreepsHookWrapper) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	i, ok := h.hookImpl.(FreepsExecutionHook)
	if ok {
		return i.OnExecutionFinished(ctx, graphName, mainArgs, mainInput)
	}
	return nil
}

func (h *FreepsHookWrapper) OnGraphChanged(ctx *base.Context, addedGraphName []string, removedGraphName []string) error {
	i, ok := h.hookImpl.(FreepsGraphChangedHook)
	if ok {
		return i.OnGraphChanged(ctx, addedGraphName, removedGraphName)
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
