package freepsgraph

import "github.com/hannesrauhe/freeps/base"

type FreepsHook interface {
	GetName() string
	OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
	OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error
	OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *GraphOperationDesc) error
	OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error
	OnGraphChanged(addedGraphName []string, removedGraphName []string) error
	Shutdown() error
}
