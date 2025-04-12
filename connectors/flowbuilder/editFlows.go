package flowbuilder

import (
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
)

// FlowFromStoreArgs are the arguments for the FlowBuilder function
type FlowFromStoreArgs struct {
	FlowName        string
	CreateIfMissing *bool
}

// FlowNameSuggestions returns suggestions for flow names
func (arg *FlowFromStoreArgs) FlowNameSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) []string {
	flowNames := []string{}
	res := freepsstore.GetFlowStore().GetAllValues(30)
	for name := range res {
		flowNames = append(flowNames, name)
	}
	return flowNames
}

func (m *OpFlowBuilder) buildDefaultOperation() freepsflow.FlowOperationDesc {
	return freepsflow.FlowOperationDesc{
		Operator:  "system",
		Function:  "noop",
		Arguments: base.MakeEmptyFunctionArguments(),
	}
}

// RestoreDeletedFlowFromStore restores a flow from the backup in store
func (m *OpFlowBuilder) RestoreDeletedFlowFromStore(ctx *base.Context, input *base.OperatorIO, args FlowFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow("deleted_" + args.FlowName)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore flow: %v", err)
	}
	err = m.GE.AddFlow(ctx, args.FlowName, gd, false)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore flow: %v", err)
	}
	return base.MakeEmptyOutput()
}

// ExecuteFlowFromStore executes a flow after loading it from the store
func (m *OpFlowBuilder) ExecuteFlowFromStore(ctx *base.Context, input *base.OperatorIO, args FlowFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found in store: %v", err)
	}
	return m.GE.ExecuteAdHocFlow(ctx, "ExecuteFromStore/"+args.FlowName, gd, base.MakeEmptyFunctionArguments(), input)
}

// GetFlowFromStore returns a flow from the store
func (m *OpFlowBuilder) GetFlowFromStore(ctx *base.Context, input *base.OperatorIO, args FlowFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err != nil {
		gd = freepsflow.FlowDesc{}
		if args.CreateIfMissing == nil || !*args.CreateIfMissing {
			return base.MakeOutputError(404, "Flow not found in store: %v", err)
		}
		return freepsstore.StoreFlow(args.FlowName, gd, ctx)
	}
	return base.MakeObjectOutput(gd)
}

// CreateFlowInStore creates a new flow in the store
func (m *OpFlowBuilder) CreateFlowInStore(ctx *base.Context, input *base.OperatorIO, args FlowFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err == nil {
		return base.MakeOutputError(400, "Flow already exists in store: %v", args.FlowName)
	}
	gd = freepsflow.FlowDesc{}
	return freepsstore.StoreFlow(args.FlowName, gd, ctx)
}
