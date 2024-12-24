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

// FlowFromStoreArgs are the arguments for the FlowBuilder function
type FlowFromStoreArgs struct {
	FlowName        string
	CreateIfMissing *bool
}

// FlowNameSuggestions returns suggestions for flow names
func (arg *FlowFromStoreArgs) FlowNameSuggestions(m *OpFlowBuilder) []string {
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

// GetFlowFromStore returns a flow from the store
func (m *OpFlowBuilder) GetFlowFromStore(ctx *base.Context, input *base.OperatorIO, args FlowFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err != nil {
		gd = freepsflow.FlowDesc{
			Operations: []freepsflow.FlowOperationDesc{
				m.buildDefaultOperation(),
			},
		}
		if args.CreateIfMissing == nil || !*args.CreateIfMissing {
			return base.MakeOutputError(404, "Flow not found in store: %v", err)
		}
		freepsstore.StoreFlow(args.FlowName, gd, ctx)
	}
	return base.MakeObjectOutput(gd)
}

// SetOperationArgs sets the fields of an operation given by the number in a flow in the store
type SetOperationArgs struct {
	FlowName        string
	OperationNumber int
	Operator        *string
	Function        *string
	ArgumentName    *string
	ArgumentValue   *string
}

// SetOperation sets the fields of an operation given by the number in a flow in the store
func (m *OpFlowBuilder) SetOperation(ctx *base.Context, input *base.OperatorIO, args SetOperationArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found in store: %v", err)
	}
	if args.OperationNumber < 0 || args.OperationNumber > len(gd.Operations) {
		return base.MakeOutputError(400, "Invalid operation number")
	}
	if args.OperationNumber == len(gd.Operations) {
		gd.Operations = append(gd.Operations, m.buildDefaultOperation())
	}

	if args.Operator != nil {
		gd.Operations[args.OperationNumber].Operator = *args.Operator
	}
	if args.Function != nil {
		gd.Operations[args.OperationNumber].Function = *args.Function
	}
	if args.ArgumentName != nil {
		if args.ArgumentValue == nil {
			return base.MakeOutputError(400, "Argument value is missing")
		}
		gd.Operations[args.OperationNumber].Arguments[*args.ArgumentName] = *args.ArgumentValue
	}
	freepsstore.StoreFlow(args.FlowName, gd, ctx)
	return base.MakeEmptyOutput()
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

// AddFlow adds a flow to the flow engine (unsused)
// func (m *OpFlowBuilder) AddFlow(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
// 	if !input.IsFormData() {
// 		return base.MakeOutputError(http.StatusBadRequest, "Invalid input format")
// 	}

// 	formdata, err := input.ParseFormData()
// 	if err != nil {
// 		return base.MakeOutputError(http.StatusBadRequest, "Invalid form data: %v", err)
// 	}

// 	flowName := formdata.Get("FlowName")
// 	if flowName == "" {
// 		return base.MakeOutputError(http.StatusBadRequest, "Flow name is missing")
// 	}
// 	overwrite, _ := utils.ConvertToBool(formdata.Get("Overwrite"))
// 	save, _ := utils.ConvertToBool(formdata.Get("SaveFlow"))

// 	gd := freepsflow.FlowDesc{}
// 	err = json.Unmarshal([]byte(formdata.Get("FlowJSON")), &gd)
// 	if err != nil {
// 		return base.MakeOutputError(http.StatusBadRequest, "Invalid flow JSON: %v", err)
// 	}

// 	if !save {
// 		output := freepsstore.StoreFlow("added_"+flowName, gd, ctx)
// 		return output
// 	} else {
// 		err = m.GE.AddFlow(flowName, gd, overwrite)
// 		if err != nil {
// 			return base.MakeOutputError(400, "Could not add flow: %v", err)
// 		}
// 	}
// 	return base.MakeEmptyOutput()
// }
