package freepsgraph

import (
	"github.com/hannesrauhe/freeps/base"
)

func (ge *GraphEngine) prepareGraphExecution(ctx *base.Context, graphName string) (*Graph, *base.OperatorIO) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gi, exists := ge.getGraphDescUnlocked(graphName)
	if !exists {
		return nil, base.MakeOutputError(404, "No graph with name \"%s\" found", graphName)
	}
	g, err := NewGraph(ctx, graphName, gi, ge)
	if err != nil {
		return nil, base.MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	return g, base.MakeEmptyOutput()
}

// ExecuteAdHocGraph executes a graph directly
func (ge *GraphEngine) ExecuteAdHocGraph(ctx *base.Context, fullName string, gd GraphDesc, mainArgs map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	g, err := NewGraph(ctx, fullName, &gd, ge)
	if err != nil {
		return base.MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	ge.TriggerOnExecuteHooks(ctx, fullName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, fullName, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	g, o := ge.prepareGraphExecution(ctx, graphName)
	if g == nil {
		return o
	}
	ge.TriggerOnExecuteHooks(ctx, graphName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, graphName, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}
