package flowbuilder

import (
	"fmt"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
)

/*
This file contains the helpers and functions to work with flows that are registered in the flow
engine (as opposed to the draft flows in the store). Flows in the engine are persisted in the
"graphs" directory of the config directory and can be executed directly.

All mutating functions of the flowbuilder can work on either layer, controlled by the "Live"
argument: if Live is true the flow is read from and written to the engine, otherwise the draft
in the store is used (which is the behaviour of the UI flow editor).
*/

// loadFlow reads a flow either from the flow engine (live=true) or from the store (live=false)
func (m *OpFlowBuilder) loadFlow(flowName string, live bool) (freepsflow.FlowDesc, error) {
	if live {
		gd, exists := m.GE.GetFlowDesc(flowName)
		if !exists || gd == nil {
			return freepsflow.FlowDesc{}, fmt.Errorf("Flow \"%s\" not found in engine", flowName)
		}
		return *gd, nil
	}
	return freepsstore.GetFlow(flowName)
}

// saveFlow writes a flow either to the flow engine (live=true, which also persists it to the
// graphs directory) or to the store (live=false)
func (m *OpFlowBuilder) saveFlow(ctx *base.Context, flowName string, gd freepsflow.FlowDesc, live bool) *base.OperatorIO {
	if live {
		if err := m.GE.AddFlow(ctx, flowName, gd, true); err != nil {
			return base.MakeOutputError(400, "Could not store flow in engine: %v", err)
		}
		return base.MakeObjectOutput(gd)
	}
	return freepsstore.StoreFlow(flowName, gd, ctx)
}

// CreateFlowArgs are the arguments for the CreateFlow function
type CreateFlowArgs struct {
	FlowID    string
	Overwrite *bool
}

// FlowIDSuggestions returns suggestions for flow names that are currently in the engine
func (arg *CreateFlowArgs) FlowIDSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	res := m.GE.GetAllFlowDesc()
	suggestions := map[string]string{}
	for id, gd := range res {
		suggestions[gd.DisplayName] = id
	}
	return suggestions
}

// CreateFlow creates (or replaces) a flow directly in the flow engine. The flow definition is
// expected as JSON (a serialized FlowDesc) in the input of the call, the flowID is given as an
// argument. The flow is validated before it is added and is persisted in the graphs directory,
// so that it survives a restart and can be executed with /flow/<flowID> .
func (m *OpFlowBuilder) CreateFlow(ctx *base.Context, input *base.OperatorIO, args CreateFlowArgs) *base.OperatorIO {
	if args.FlowID == "" {
		return base.MakeOutputError(400, "No flowID given")
	}
	gd := freepsflow.FlowDesc{}
	if err := input.ParseJSON(&gd); err != nil {
		return base.MakeOutputError(400, "Could not parse flow definition from input: %v", err)
	}
	if len(gd.Operations) == 0 {
		return base.MakeOutputError(400, "Flow definition contains no operations")
	}
	overwrite := args.Overwrite != nil && *args.Overwrite
	if err := m.GE.AddFlow(ctx, args.FlowID, gd, overwrite); err != nil {
		return base.MakeOutputError(400, "Could not create flow: %v", err)
	}
	return base.MakeObjectOutput(gd)
}

// ListFlowsArgs are the arguments for the ListFlows function
type ListFlowsArgs struct {
	// Tags is an optional comma separated list of tags. If given, only flows that carry all
	// of the given tags are returned.
	Tags *string
}

// ListFlows returns the flow descriptions of all flows that are currently registered in the flow
// engine, or only the ones with a given tag if the Tags argument is set.
func (m *OpFlowBuilder) ListFlows(ctx *base.Context, input *base.OperatorIO, args ListFlowsArgs) *base.OperatorIO {
	if args.Tags != nil && *args.Tags != "" {
		return base.MakeObjectOutput(m.GE.GetFlowDescByTag(strings.Split(*args.Tags, ",")))
	}
	return base.MakeObjectOutput(m.GE.GetAllFlowDesc())
}
