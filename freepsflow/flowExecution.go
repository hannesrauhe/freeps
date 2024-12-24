package freepsflow

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

func (ge *FlowEngine) prepareFlowExecution(ctx *base.Context, flowName string) (*Flow, *base.OperatorIO) {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()
	gi, exists := ge.getFlowDescUnlocked(flowName)
	if !exists {
		return nil, base.MakeOutputError(404, "No flow with name \"%s\" found", flowName)
	}
	g, err := NewFlow(ctx, flowName, gi, ge)
	if err != nil {
		return nil, base.MakeOutputError(500, "Flow preparation failed: "+err.Error())
	}
	return g, base.MakeEmptyOutput()
}

// ExecuteAdHocFlow executes a flow directly
func (ge *FlowEngine) ExecuteAdHocFlow(ctx *base.Context, fullName string, gd FlowDesc, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g, err := NewFlow(ctx, fullName, &gd, ge)
	if err != nil {
		return base.MakeOutputError(500, "Flow preparation failed: "+err.Error())
	}
	ge.TriggerOnExecuteHooks(ctx, fullName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, fullName, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteFlow executes a flow stored in the engine
func (ge *FlowEngine) ExecuteFlow(ctx *base.Context, flowName string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g, o := ge.prepareFlowExecution(ctx, flowName)
	if g == nil {
		return o
	}
	ge.TriggerOnExecuteHooks(ctx, flowName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, flowName, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteOperatorByName executes an operator directly
func (ge *FlowEngine) ExecuteOperatorByName(ctx *base.Context, opName string, fn string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	name := fmt.Sprintf("OnDemand/%v/%v", opName, fn)
	return ge.ExecuteAdHocFlow(ctx, name, FlowDesc{Operations: []FlowOperationDesc{{Operator: opName, Function: fn, UseMainArgs: true}}}, mainArgs, mainInput)
}

// ExecuteFlowByTags executes flows with given tags
func (ge *FlowEngine) ExecuteFlowByTags(ctx *base.Context, tags []string, args base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	taggroups := [][]string{}
	for _, t := range tags {
		taggroups = append(taggroups, []string{t})
	}
	return ge.ExecuteFlowByTagsExtended(ctx, taggroups, args, input)
}

// ExecuteFlowByTagsExtended executes all flows that at least one tag of each group
func (ge *FlowEngine) ExecuteFlowByTagsExtended(ctx *base.Context, tagGroups [][]string, args base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	if tagGroups == nil || len(tagGroups) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "No tags given")
	}

	// ctx.GetLogger().Infof("Executing flow by tags: %v", tagGroups)

	tg := ge.GetFlowDescByTagExtended(tagGroups...)
	if len(tg) <= 1 {
		for n := range tg {
			return ge.ExecuteFlow(ctx, n, args, input)
		}
		return base.MakeOutputError(404, "No flow with tags found: %v", fmt.Sprint(tagGroups))
	}

	// need to build a temporary flow containing all flows with matching tags
	op := []FlowOperationDesc{}
	for n := range tg {
		op = append(op, FlowOperationDesc{Name: n, Operator: "flow", Function: n, InputFrom: "_", UseMainArgs: true})
	}
	gd := FlowDesc{Operations: op, Tags: []string{"internal"}}
	name := fmt.Sprintf("ExecuteFlowByTag/%v", tagGroups)

	return ge.ExecuteAdHocFlow(ctx, name, gd, args, input)
}
