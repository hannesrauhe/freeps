package freepsflow

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/base"
)

func (ge *FlowEngine) prepareFlowExecution(ctx *base.Context, flowName string) (*Flow, *base.OperatorIO) {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()
	if flowName == "" {
		names := make([]string, 0, len(ge.flows))
		for n := range ge.flows {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, base.MakeOutputError(400, "No flow given. Available flows: %s", strings.Join(names, ", "))
	}
	gi, exists := ge.getFlowDescUnlocked(flowName)
	if !exists {
		return nil, base.MakeOutputError(404, "No flow with name \"%s\" found", flowName)
	}
	g, err := NewFlow(ctx, flowName, gi, ge)
	if err != nil {
		return nil, base.MakeOutputError(500, "Flow preparation failed: %s", err.Error())
	}
	return g, base.MakeEmptyOutput()
}

// rejectDeactivated rejects the execution of a deactivated flow. Calling a flow that is switched
// off is almost always a mistake, so this raises an alert as well - otherwise a flow that gets
// deactivated by accident would simply stop working without anybody noticing.
//
// It must be called without holding flowLock: the alert triggers flows again, which would
// deadlock on the (non reentrant) lock.
func (ge *FlowEngine) rejectDeactivated(ctx *base.Context, flowName string, mainArgs base.FunctionArguments) *base.OperatorIO {
	err := fmt.Errorf("Flow \"%s\" is deactivated and was called with arguments \"%v\"", flowName, mainArgs)
	ctx.GetLogger().Warnf(err.Error())
	ge.SetSystemAlert(ctx, fmt.Sprintf("deactivated.%s", flowName), "system", 2, err, &ge.config.AlertDuration)
	return base.MakeOutputError(http.StatusForbidden, "%v", err)
}

// ExecuteAdHocFlow executes a flow directly
func (ge *FlowEngine) ExecuteAdHocFlow(ctx *base.Context, fullName string, gd FlowDesc, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	if gd.IsDeactivated() {
		return ge.rejectDeactivated(ctx, fullName, mainArgs)
	}
	g, err := NewFlow(ctx, fullName, &gd, ge)
	if err != nil {
		return base.MakeOutputError(500, "Flow preparation failed: %s", err.Error())
	}
	ge.TriggerOnExecuteHooks(ctx, fullName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, fullName, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteFlow executes a flow stored in the engine
func (ge *FlowEngine) ExecuteFlow(ctx *base.Context, flowName string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	if gd, exists := ge.GetFlowDesc(flowName); exists && gd.IsDeactivated() {
		return ge.rejectDeactivated(ctx, flowName, mainArgs)
	}
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
	// Without this check the ad-hoc flow validation would report an unknown operator as a
	// 500 "Flow preparation failed"; a direct call by name deserves a 404 with the list.
	if !ge.HasOperator(opName) {
		return base.MakeOutputError(404, "No operator with name \"%s\" found. Available operators: %s", opName, strings.Join(ge.GetOperators(), ", "))
	}
	name := fmt.Sprintf("OnDemand/%v/%v", opName, fn)
	return ge.ExecuteAdHocFlow(ctx, name, FlowDesc{Operations: []FlowOperationDesc{{Operator: opName, Function: fn, UseMainArgs: true, InputFrom: ROOT_SYMBOL}}}, mainArgs, mainInput)
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
