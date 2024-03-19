package freepsgraph

import (
	"fmt"
	"net/http"

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
func (ge *GraphEngine) ExecuteAdHocGraph(ctx *base.Context, fullName string, gd GraphDesc, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g, err := NewGraph(ctx, fullName, &gd, ge)
	if err != nil {
		return base.MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	ge.TriggerOnExecuteHooks(ctx, fullName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, fullName, mainArgs, mainInput)
	return g.ExecuteOld(ctx, mainArgs, mainInput)
}

// ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(ctx *base.Context, graphName string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g, o := ge.prepareGraphExecution(ctx, graphName)
	if g == nil {
		return o
	}
	ge.TriggerOnExecuteHooks(ctx, graphName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, graphName, mainArgs, mainInput)
	return g.ExecuteOld(ctx, mainArgs, mainInput)
}

// ExecuteOperatorByName executes an operator directly
func (ge *GraphEngine) ExecuteOperatorByName(ctx *base.Context, opName string, fn string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	name := fmt.Sprintf("OnDemand/%v/%v", opName, fn)
	return ge.ExecuteAdHocGraph(ctx, name, GraphDesc{Operations: []GraphOperationDesc{{Operator: opName, Function: fn, UseMainArgs: true}}}, mainArgs, mainInput)
}

// ExecuteGraphByTags executes graphs with given tags
func (ge *GraphEngine) ExecuteGraphByTags(ctx *base.Context, tags []string, args base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	taggroups := [][]string{}
	for _, t := range tags {
		taggroups = append(taggroups, []string{t})
	}
	return ge.ExecuteGraphByTagsExtended(ctx, taggroups, args, input)
}

// ExecuteGraphByTagsExtended executes all graphs that at least one tag of each group
func (ge *GraphEngine) ExecuteGraphByTagsExtended(ctx *base.Context, tagGroups [][]string, args base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	if tagGroups == nil || len(tagGroups) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "No tags given")
	}

	// ctx.GetLogger().Infof("Executing graph by tags: %v", tagGroups)

	tg := ge.GetGraphDescByTagExtended(tagGroups...)
	if len(tg) <= 1 {
		for n := range tg {
			return ge.ExecuteGraph(ctx, n, args, input)
		}
		return base.MakeOutputError(404, "No graph with tags found: %v", fmt.Sprint(tagGroups))
	}

	// need to build a temporary graph containing all graphs with matching tags
	op := []GraphOperationDesc{}
	for n := range tg {
		op = append(op, GraphOperationDesc{Name: n, Operator: "graph", Function: n, InputFrom: "_", UseMainArgs: true})
	}
	gd := GraphDesc{Operations: op, Tags: []string{"internal"}}
	name := fmt.Sprintf("ExecuteGraphByTag/%v", tagGroups)

	return ge.ExecuteAdHocGraph(ctx, name, gd, args, input)
}
