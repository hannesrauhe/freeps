package freepsgraph

import (
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/base"
)

type OpGraph struct {
	ge *GraphEngine
}

type OpGraphByTag struct {
	ge *GraphEngine
}

var _ base.FreepsBaseOperator = &OpGraph{}
var _ base.FreepsBaseOperator = &OpGraphByTag{}

func (o *OpGraph) Execute(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
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

// GetPossibleArgs returns suggestions based on the suggestions of the operators in the graph
func (o *OpGraph) GetPossibleArgs(fn string) []string {
	agd, exists := o.ge.GetGraphDesc(fn)
	if !exists {
		return []string{}
	}
	possibleArgs := make([]string, 0)
	for _, op := range agd.Operations {
		if op.IgnoreMainArgs {
			continue
		}
		thisOpArgs := o.ge.GetOperator(op.Operator).GetPossibleArgs(op.Function)
		possibleArgs = append(possibleArgs, thisOpArgs...)
	}
	return possibleArgs
}

// GetArgSuggestions returns suggestions based on the suggestions of the operators in the graph
func (o *OpGraph) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	agd, exists := o.ge.GetGraphDesc(fn)
	if !exists {
		return map[string]string{}
	}
	possibleValues := make(map[string]string, 0)
	for _, op := range agd.Operations {
		if op.IgnoreMainArgs {
			continue
		}
		// build a map of all arguments that will be passed to this operation on execution
		thisOpArgs := make(map[string]string)
		for k, v := range op.Arguments {
			thisOpArgs[k] = v
		}
		for k, v := range otherArgs {
			thisOpArgs[k] = v
		}
		thisOpSuggestions := o.ge.GetOperator(op.Operator).GetArgSuggestions(op.Function, arg, thisOpArgs)
		for k, v := range thisOpSuggestions {
			possibleValues[k] = v
		}
	}
	return possibleValues
}

// StartListening (noOp)
func (o *OpGraph) StartListening(*base.Context) {
}

// Shutdown (noOp)
func (o *OpGraph) Shutdown(*base.Context) {
}

/*** By Tag ****/

func (o *OpGraphByTag) Execute(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	if input.IsError() { // graph has been called by another operator, but the operator returned an error
		return input
	}
	tags := []string{}
	if fn != "" {
		tags = append(tags, fn)
	}
	addTstr := args["additionalTags"]
	if addTstr != "" {
		tags = append(tags, strings.Split(addTstr, ",")...)
	}

	return o.ge.ExecuteGraphByTags(ctx, tags, make(map[string]string), base.MakeEmptyOutput())
}

// GetName returns the name of the operator
func (o *OpGraphByTag) GetName() string {
	return "graphbytag"
}

// GetFunctions returns a list of all available tags
func (o *OpGraphByTag) GetFunctions() []string {
	agd := o.ge.GetTags()
	graphs := make([]string, 0, len(agd))
	for n := range agd {
		graphs = append(graphs, n)
	}
	sort.Strings(graphs)
	return graphs
}

// GetPossibleArgs returns the additonalTags Option
func (o *OpGraphByTag) GetPossibleArgs(fn string) []string {
	return []string{"additionalTags"}
}

// GetArgSuggestions returns addtional tags
func (o *OpGraphByTag) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return o.ge.GetTags()
}

// StartListening (noOp)
func (o *OpGraphByTag) StartListening(*base.Context) {
}

// Shutdown (noOp)
func (o *OpGraphByTag) Shutdown(*base.Context) {
}
