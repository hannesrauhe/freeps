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
		Arguments: map[string]string{},
	}
}

// RestoreDeletedFlowFromStore restores a flow from the backup in store
func (m *OpFlowBuilder) RestoreDeletedFlowFromStore(ctx *base.Context, input *base.OperatorIO, args FlowFromStoreArgs) *base.OperatorIO {
	return m.promoteFlowFromStore(ctx, "deleted_"+args.FlowName, args.FlowName)
}

// PromoteFlowArgs are the arguments for the PromoteFlow function
type PromoteFlowArgs struct {
	// FlowName is the name the flow gets in the flow engine
	FlowName string
	// StoreName is the key the draft flow is stored under in the store. Defaults to FlowName.
	StoreName *string
	// Overwrite allows replacing an existing flow in the engine
	Overwrite *bool
}

// PromoteFlow takes a (draft) flow from the store and registers it in the flow engine, which
// validates it and persists it in the graphs directory. This is the step that makes a flow that
// was built programmatically (or in the UI editor) permanent and executable with /flow/<name> .
func (m *OpFlowBuilder) PromoteFlow(ctx *base.Context, input *base.OperatorIO, args PromoteFlowArgs) *base.OperatorIO {
	storeName := args.FlowName
	if args.StoreName != nil && *args.StoreName != "" {
		storeName = *args.StoreName
	}
	overwrite := args.Overwrite != nil && *args.Overwrite
	gd, err := freepsstore.GetFlow(storeName)
	if err != nil {
		return base.MakeOutputError(404, "Could not load flow from store: %v", err)
	}
	if err := m.GE.AddFlow(ctx, args.FlowName, gd, overwrite); err != nil {
		return base.MakeOutputError(400, "Could not add flow to engine: %v", err)
	}
	return base.MakeObjectOutput(gd)
}

// promoteFlowFromStore is a small helper for the case where store name and flow name only differ
// by a prefix (used by RestoreDeletedFlowFromStore)
func (m *OpFlowBuilder) promoteFlowFromStore(ctx *base.Context, storeName string, flowName string) *base.OperatorIO {
	noOverwrite := false
	return m.PromoteFlow(ctx, base.MakeEmptyOutput(), PromoteFlowArgs{FlowName: flowName, StoreName: &storeName, Overwrite: &noOverwrite})
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
