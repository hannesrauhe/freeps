package flowbuilder

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
)

// OpFlowBuilder is the operator to build and modify flows
type OpFlowBuilder struct {
	GE *freepsflow.FlowEngine
}

var _ base.FreepsOperator = &OpFlowBuilder{}

// FlowFromEngineArgs are the arguments for the FlowBuilder function
type FlowFromEngineArgs struct {
	FlowID string
}

// FlowIDsuggestions returns suggestions for flow names
func (arg *FlowFromEngineArgs) FlowidSuggestions(m *OpFlowBuilder) map[string]string {
	flowNames := map[string]string{}
	res := m.GE.GetAllFlowDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, m.GE)
		_, exists := flowNames[info.DisplayName]
		if !exists {
			flowNames[info.DisplayName] = id
		} else {
			flowNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return flowNames
}

// GetFlow returns a flow from the flow engine
func (m *OpFlowBuilder) GetFlow(ctx *base.Context, input *base.OperatorIO, args FlowFromEngineArgs) *base.OperatorIO {
	gd, ok := m.GE.GetFlowDesc(args.FlowID)
	if !ok {
		return base.MakeOutputError(404, "Flow not found in Engine: %v", args.FlowID)
	}
	return base.MakeObjectOutput(gd)
}

// DeleteFlow deletes a flow from the flow engine and stores a backup in the store
func (m *OpFlowBuilder) DeleteFlow(ctx *base.Context, input *base.OperatorIO, args FlowFromEngineArgs) *base.OperatorIO {
	backup, err := m.GE.DeleteFlow(ctx, args.FlowID)
	if backup != nil {
		freepsstore.StoreFlow("deleted_"+args.FlowID, *backup, ctx)
	}
	if err != nil {
		return base.MakeOutputError(400, "Could not delete flow: %v", err)
	}

	return base.MakeEmptyOutput()
}
