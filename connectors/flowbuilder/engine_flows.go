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
engine (as opposed to the draft flows in the store). Flows in the engine are persisted in the config directory and can be executed directly.

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
// config directory) or to the store (live=false)
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
// argument. The flow is validated before it is added and is persisted,
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
	Tags    *string  `doc:"comma separated tags, only flows with all of them are returned"`
	Kind    []string `doc:"kinds (manual, helper, event), repeat the argument for multiple. Only flows of one of the given kinds are returned, flows without a kind count as manual." options:"manual,helper,event"`
	Details *bool    `doc:"return the full definitions including the operations, instead of the brief description"`
}

// ListFlows returns all flows in the flow engine, filtered by Tags and/or Kind. By default only
// the brief description (DisplayName, Description, Kind, Tags) is returned, use Details=true for
// the full definitions.
func (m *OpFlowBuilder) ListFlows(ctx *base.Context, input *base.OperatorIO, args ListFlowsArgs) *base.OperatorIO {
	flows := map[string]freepsflow.FlowDesc{}
	if args.Tags != nil && *args.Tags != "" {
		flows = m.GE.GetFlowDescByTag(strings.Split(*args.Tags, ","))
	} else {
		for id, gd := range m.GE.GetAllFlowDesc() {
			flows[id] = *gd
		}
	}
	if len(args.Kind) > 0 {
		kinds := make([]string, 0, len(args.Kind))
		for _, k := range args.Kind {
			kinds = append(kinds, strings.ToLower(strings.TrimSpace(k)))
		}
		filtered := map[string]freepsflow.FlowDesc{}
		for id, gd := range flows {
			for _, k := range kinds {
				if (k == freepsflow.FlowKindManual && gd.IsManual()) || strings.EqualFold(gd.Kind, k) {
					filtered[id] = gd
					break
				}
			}
		}
		flows = filtered
	}
	if args.Details != nil && *args.Details {
		return base.MakeObjectOutput(flows)
	}
	brief := map[string]freepsflow.FlowBriefDesc{}
	for id, gd := range flows {
		brief[id] = gd.Brief(id)
	}
	return base.MakeObjectOutput(brief)
}

// SetFlowDescriptionArgs are the arguments for the SetFlowDescription function
type SetFlowDescriptionArgs struct {
	FlowName    string `doc:"the name of the flow"`
	Description string `doc:"the new description, empty clears it"`
	Live        *bool  `doc:"operate on the flow in the engine (persisted) instead of the draft in the store"`
}

// SetFlowDescription sets the Description of a flow in the store (or in the flow engine if Live
// is set) without touching the operations. An empty description clears it.
func (m *OpFlowBuilder) SetFlowDescription(ctx *base.Context, input *base.OperatorIO, args SetFlowDescriptionArgs) *base.OperatorIO {
	live := args.Live != nil && *args.Live
	gd, err := m.loadFlow(args.FlowName, live)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found: %v", err)
	}
	gd.Description = args.Description
	return m.saveFlow(ctx, args.FlowName, gd, live)
}

// SetFlowKindArgs are the arguments for the SetFlowKind function
type SetFlowKindArgs struct {
	FlowName string `doc:"the name of the flow"`
	Kind     string `doc:"manual, helper or event. Empty resets to the default (manual)." options:"manual,helper,event"`
	Live     *bool  `doc:"operate on the flow in the engine (persisted) instead of the draft in the store"`
}

// SetFlowKind sets the Kind of a flow in the store (or in the flow engine if Live is set)
// without touching the operations. Valid kinds are "manual", "helper" and "event"; an empty
// kind resets it to the default ("manual").
func (m *OpFlowBuilder) SetFlowKind(ctx *base.Context, input *base.OperatorIO, args SetFlowKindArgs) *base.OperatorIO {
	live := args.Live != nil && *args.Live
	kind := strings.ToLower(strings.TrimSpace(args.Kind))
	switch kind {
	case "", freepsflow.FlowKindManual, freepsflow.FlowKindHelper, freepsflow.FlowKindEvent:
	default:
		return base.MakeOutputError(400, "Invalid kind \"%s\", valid kinds are manual, helper and event", args.Kind)
	}
	gd, err := m.loadFlow(args.FlowName, live)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found: %v", err)
	}
	gd.Kind = kind
	return m.saveFlow(ctx, args.FlowName, gd, live)
}
