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

func (m *OpSystem) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "shutdown":
		m.ge.reloadRequested = false
		m.cancel()
	case "reload":
		m.ge.reloadRequested = true
		m.cancel()
	}
	return MakeEmptyOutput()
}
