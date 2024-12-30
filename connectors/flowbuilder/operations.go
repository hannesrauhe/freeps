package flowbuilder

import (
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
)

// AddOperationArgs are the arguments for the AddOperation function, number and all OperationDesc fields are optional
type AddOperation struct {
	FlowName           string
	OperationNumber    *int
	OperationName      *string
	Operator           *string
	Function           *string
	InputFrom          *string
	ExecuteOnSuccessOf *string
	ExecuteOnFailOf    *string
	ArgumentsFrom      *string
	UseMainArgs        *bool
}

// AddOperation adds an operation to a flow in the store
func (m *OpFlowBuilder) AddOperation(ctx *base.Context, input *base.OperatorIO, args AddOperation) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found in store: %v", err)
	}
	operationNumber := len(gd.Operations)
	if args.OperationNumber != nil {
		operationNumber = *args.OperationNumber
	}

	operationDesc := m.buildDefaultOperation()
	if args.OperationName != nil {
		operationDesc.Name = *args.OperationName
	}
	if args.Operator != nil {
		operationDesc.Operator = *args.Operator
	}
	if args.Function != nil {
		operationDesc.Function = *args.Function
	}
	if args.InputFrom != nil {
		operationDesc.InputFrom = *args.InputFrom
	}
	if args.ExecuteOnSuccessOf != nil {
		operationDesc.ExecuteOnSuccessOf = *args.ExecuteOnSuccessOf
	}
	if args.ExecuteOnFailOf != nil {
		operationDesc.ExecuteOnFailOf = *args.ExecuteOnFailOf
	}
	if args.ArgumentsFrom != nil {
		operationDesc.ArgumentsFrom = *args.ArgumentsFrom
	}
	if args.UseMainArgs != nil {
		operationDesc.UseMainArgs = *args.UseMainArgs
	}
	if operationNumber < 0 || operationNumber > len(gd.Operations) {
		gd.Operations = append(gd.Operations, operationDesc)
	} else {
		gd.Operations = append(gd.Operations[:operationNumber], append([]freepsflow.FlowOperationDesc{operationDesc}, gd.Operations[operationNumber:]...)...)
	}
	return freepsstore.StoreFlow(args.FlowName, gd, ctx)
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
	return freepsstore.StoreFlow(args.FlowName, gd, ctx)
}

// RemoveOperationArgs are the arguments for the RemoveOperation function
type RemoveOperationArgs struct {
	FlowName        string
	OperationNumber int
}

// RemoveOperation removes an operation from a flow in the store
func (m *OpFlowBuilder) RemoveOperation(ctx *base.Context, input *base.OperatorIO, args RemoveOperationArgs) *base.OperatorIO {
	gd, err := freepsstore.GetFlow(args.FlowName)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found in store: %v", err)
	}
	if args.OperationNumber == len(gd.Operations)-1 {
		gd.Operations = gd.Operations[:args.OperationNumber]
	} else if args.OperationNumber == 0 {
		gd.Operations = gd.Operations[1:]
	} else if args.OperationNumber < len(gd.Operations)-1 {
		gd.Operations = append(gd.Operations[:args.OperationNumber], gd.Operations[args.OperationNumber+1:]...)
	} else {
		return base.MakeOutputError(400, "Invalid operation number")
	}
	return freepsstore.StoreFlow(args.FlowName, gd, ctx)
}
