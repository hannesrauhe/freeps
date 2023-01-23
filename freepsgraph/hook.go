package freepsgraph

import "github.com/hannesrauhe/freeps/utils"

type FreepsHook interface {
	GetName() string
	OnExecute(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) error
	OnExecutionFinished(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) error
	Shutdown() error
}
