package freepsgraph

import (
	"net/http"
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/utils"
)

type OpGraph struct {
	ge *GraphEngine
}

type OpGraphByTag struct {
	ge *GraphEngine
}

var _ FreepsOperator = &OpGraph{}
var _ FreepsOperator = &OpGraphByTag{}

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

/*** By Tag ****/

func (o *OpGraphByTag) Execute(ctx *utils.Context, fn string, args map[string]string, input *OperatorIO) *OperatorIO {
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

	if len(tags) == 0 {
		return MakeOutputError(http.StatusBadRequest, "No tags given")
	}

	tg := o.ge.GetGraphInfoByTag(tags)
	if len(tg) <= 1 {
		for n := range tg {
			return o.ge.ExecuteGraph(ctx, n, args, input)
		}
		return MakeOutputError(404, "No graph with tags \"%s\" found", strings.Join(tags, ","))
	}
	// need to build a temporary graph containing all graphs
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

// Shutdown (noOp)
func (o *OpGraphByTag) Shutdown(*utils.Context) {
}
