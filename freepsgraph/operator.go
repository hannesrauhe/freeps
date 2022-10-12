package freepsgraph

import "sort"

type FreepsOperator interface {
	// GetOutputType() OutputT
	Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO

	GetFunctions() []string // returns a list of functions that this operator can execute
	GetPossibleArgs(fn string) []string
	GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string
}

type OpGraph struct {
	ge *GraphEngine
}

var _ FreepsOperator = &OpGraph{}

func (o *OpGraph) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	if input.IsError() { // graph has been called by another operator, but the operator returned an error
		return input
	}
	return o.ge.ExecuteGraph(fn, args, input)
}

// GetFunctions returns a list of graphs stored in the engine
func (o *OpGraph) GetFunctions() []string {
	agd := o.ge.GetAllGraphDesc()
	graphs := make([]string, 0, len(agd))
	for n := range agd {
		graphs = append(graphs, n)
	}
	sort.Strings(graphs)
	return graphs
}

func (o *OpGraph) GetPossibleArgs(fn string) []string {
	return []string{}
}

func (o *OpGraph) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}
