package freepsgraph

type FreepsOperator interface {
	// GetOutputType() OutputT
	Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO
}

type OpGraph struct {
	ge *GraphEngine
}

var _ FreepsOperator = &OpGraph{}

func (o *OpGraph) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	return o.ge.ExecuteGraph(fn, mainArgs, mainInput)
}

// Operators: OR, AND, PARALLEL, NOT(?), InputTransform
