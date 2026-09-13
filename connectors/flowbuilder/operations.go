package flowbuilder

import (
	"github.com/hannesrauhe/freeps/base"
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
	// Live makes the operation work on the flow in the flow engine (which is persisted in the
	// config directory) instead of the draft flow in the store
	Live *bool
}

// AddOperation adds an operation to a flow in the store (or in the flow engine if Live is set)
func (m *OpFlowBuilder) AddOperation(ctx *base.Context, input *base.OperatorIO, args AddOperation) *base.OperatorIO {
	live := args.Live != nil && *args.Live
	gd, err := m.loadFlow(args.FlowName, live)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found: %v", err)
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
	return m.saveFlow(ctx, args.FlowName, gd, live)
}

// SetOperationArgs sets the fields of an operation given by the number in a flow in the store
type SetOperationArgs struct {
	FlowName        string
	OperationNumber int
	Operator        *string
	Function        *string
	ArgumentName    *string
	ArgumentValue   *string
	// Live makes the operation work on the flow in the flow engine (which is persisted in the
	// config directory) instead of the draft flow in the store
	Live *bool
}

// SetOperation sets the fields of an operation given by the number in a flow in the store (or in the flow engine if Live is set)
func (m *OpFlowBuilder) SetOperation(ctx *base.Context, input *base.OperatorIO, args SetOperationArgs) *base.OperatorIO {
	live := args.Live != nil && *args.Live
	gd, err := m.loadFlow(args.FlowName, live)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found: %v", err)
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
		if gd.Operations[args.OperationNumber].Arguments == nil {
			gd.Operations[args.OperationNumber].Arguments = map[string]string{}
		}
		gd.Operations[args.OperationNumber].Arguments[*args.ArgumentName] = *args.ArgumentValue
	}
	return m.saveFlow(ctx, args.FlowName, gd, live)
}

// RemoveOperationArgs are the arguments for the RemoveOperation function
type RemoveOperationArgs struct {
	FlowName        string
	OperationNumber int
	// Live makes the operation work on the flow in the flow engine (which is persisted in the
	// config directory) instead of the draft flow in the store
	Live *bool
}

// RemoveOperation removes an operation from a flow in the store (or in the flow engine if Live is set)
func (m *OpFlowBuilder) RemoveOperation(ctx *base.Context, input *base.OperatorIO, args RemoveOperationArgs) *base.OperatorIO {
	live := args.Live != nil && *args.Live
	gd, err := m.loadFlow(args.FlowName, live)
	if err != nil {
		return base.MakeOutputError(404, "Flow not found: %v", err)
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
	return m.saveFlow(ctx, args.FlowName, gd, live)
}
