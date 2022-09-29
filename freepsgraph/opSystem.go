package freepsgraph

import (
	"context"
)

type OpSystem struct {
	ge     *GraphEngine
	cancel context.CancelFunc
}

var _ FreepsOperator = &OpSystem{}

func NewSytemOp(ge *GraphEngine, cancel context.CancelFunc) *OpSystem {
	return &OpSystem{ge: ge, cancel: cancel}
}

func (o *OpSystem) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "shutdown":
		o.ge.reloadRequested = false
		o.cancel()
	case "reload":
		o.ge.reloadRequested = true
		o.cancel()
	}
	return MakeEmptyOutput()
}

func (o *OpSystem) GetFunctions() []string {
	return []string{"shutdown", "reload"}
}

func (o *OpSystem) GetPossibleArgs(fn string) []string {
	return []string{}
}

func (o *OpSystem) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}
