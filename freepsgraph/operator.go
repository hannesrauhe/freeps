package freepsgraph

import (
	"sort"

	"github.com/hannesrauhe/freeps/utils"
)

type FreepsOperator interface {
	// GetOutputType() OutputT
	Execute(ctx *utils.Context, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO

	GetFunctions() []string // returns a list of functions that this operator can execute
	GetPossibleArgs(fn string) []string
	GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string
	GetName() string
	Shutdown(*utils.Context)
}

type OpGraph struct {
	ge *GraphEngine
}

var _ FreepsOperator = &OpGraph{}

func (o *OpGraph) Execute(ctx *utils.Context, fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	if input.IsError() { // graph has been called by another operator, but the operator returned an error
		return input
	}
	return o.ge.ExecuteGraph(ctx, fn, args, input)
}

// GetName returns the name of the operator
func (o *OpGraph) GetName() string {
	return "graph"
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

// GetPossibleArgs returns an empty slice, because possible arguments are unknown
func (o *OpGraph) GetPossibleArgs(fn string) []string {
	return []string{}
}

// GetArgSuggestions returns an empty map, because possible arguments are unknown
func (o *OpGraph) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpGraph) Shutdown(*utils.Context) {
}
