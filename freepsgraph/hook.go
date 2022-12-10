package freepsgraph

import "github.com/hannesrauhe/freeps/utils"

type FreepsHook interface {
	OnExecute(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) error
	Shutdown() error
}
