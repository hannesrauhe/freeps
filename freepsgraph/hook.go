package freepsgraph

import "github.com/hannesrauhe/freeps/base"

type FreepsHook interface {
	GetName() string
	OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) error
	OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) error
	OnGraphChanged(addedGraphName []string, removedGraphName []string) error
	Shutdown() error
}
