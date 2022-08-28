package freepsgraph

type FreepsOperator interface {
	// GetOutputType() OutputT
	Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO
}

type OpGraph struct {
	g *Graph
}

var _ FreepsOperator = &OpGraph{}

func (o *OpGraph) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	subGraph, exists := o.g.engine.graphs[fn]
	if exists {
		return subGraph.Execute(mainArgs, mainInput)
	}
	return MakeOutputError(404, "No graph with name \"%s\" found", fn)
}

// Operators: OR, AND, PARALLEL, NOT(?), InputTransform
