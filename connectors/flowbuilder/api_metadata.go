package flowbuilder

import (
	"github.com/hannesrauhe/freeps/base"
)

// This file exposes the operator metadata that drives the flow editor UI as a
// read-only JSON API. The metadata forms a four level ladder, one endpoint per level:
//
//	operators -> functions -> arguments -> argument details
//
// so a client can walk from the top down without ever fetching more than it asked for.
// The last level combines the two kinds of detail a single argument can have: the
// description from its "doc" struct tag and the suggestions for its value.

// ListOperatorsArgs are the arguments for ListOperators (there are none).
type ListOperatorsArgs struct{}

// ListOperators returns the names of all operators that are currently registered in the engine.
func (m *OpFlowBuilder) ListOperators(ctx *base.Context, input *base.OperatorIO, args ListOperatorsArgs) *base.OperatorIO {
	return base.MakeObjectOutput(m.GE.GetOperators())
}

// ListFunctionsArgs are the arguments for ListFunctions.
type ListFunctionsArgs struct {
	Operator string
}

// OperatorSuggestions returns suggestions for the operator name.
func (arg *ListFunctionsArgs) OperatorSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	return operatorNameSuggestions(m)
}

// ListFunctions returns the names of all functions of the given operator.
func (m *OpFlowBuilder) ListFunctions(ctx *base.Context, input *base.OperatorIO, args ListFunctionsArgs) *base.OperatorIO {
	op := m.GE.GetOperator(args.Operator)
	if op == nil {
		return base.MakeOutputError(404, "Operator not found: %v", args.Operator)
	}
	return base.MakeObjectOutput(op.GetFunctions())
}

// OperatorArgsArgs are the arguments for OperatorArgs.
type OperatorArgsArgs struct {
	Operator string
	Function string
}

// OperatorSuggestions returns suggestions for the operator name.
func (arg *OperatorArgsArgs) OperatorSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	return operatorNameSuggestions(m)
}

// FunctionSuggestions returns suggestions for the function name of the selected operator.
func (arg *OperatorArgsArgs) FunctionSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	return functionNameSuggestions(m, arg.Operator)
}

// OperatorArgs returns the names of all arguments of the given function of the given operator.
func (m *OpFlowBuilder) OperatorArgs(ctx *base.Context, input *base.OperatorIO, args OperatorArgsArgs) *base.OperatorIO {
	op := m.GE.GetOperator(args.Operator)
	if op == nil {
		return base.MakeOutputError(404, "Operator not found: %v", args.Operator)
	}
	return base.MakeObjectOutput(op.GetPossibleArgs(args.Function))
}

// ArgDetailsArgs are the arguments for ArgDetails.
type ArgDetailsArgs struct {
	Operator string
	Function string
	// OtherArgs are additional arguments in URL query format, passed to the
	// suggestion functions so they can return context sensitive suggestions.
	OtherArgs *string
}

// OperatorSuggestions returns suggestions for the operator name.
func (arg *ArgDetailsArgs) OperatorSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	return operatorNameSuggestions(m)
}

// FunctionSuggestions returns suggestions for the function name of the selected operator.
func (arg *ArgDetailsArgs) FunctionSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	return functionNameSuggestions(m, arg.Operator)
}

// ArgDetail is one argument of a function, with the description from its "doc" struct tag
// and the suggestions for its value.
type ArgDetail struct {
	base.ArgumentDescription
	Suggestions map[string]string
}

// ArgDetails returns name, type, requiredness, description and value suggestions for every
// argument of the given function of the given operator. The descriptions come from the
// optional "doc" struct tags of the parameter structs and are empty where no tag is set.
// The suggestions are the same the flow editor shows in its drop down lists, so this
// endpoint replaces fetching them argument by argument.
func (m *OpFlowBuilder) ArgDetails(ctx *base.Context, input *base.OperatorIO, args ArgDetailsArgs) *base.OperatorIO {
	op := m.GE.GetOperator(args.Operator)
	if op == nil {
		return base.MakeOutputError(404, "Operator not found: %v", args.Operator)
	}
	otherArgs := base.MakeEmptyFunctionArguments()
	if args.OtherArgs != nil && *args.OtherArgs != "" {
		var err error
		otherArgs, err = base.NewFunctionArgumentsFromURLQuery(*args.OtherArgs)
		if err != nil {
			return base.MakeOutputError(400, "Could not parse otherArgs \"%v\": %v", *args.OtherArgs, err)
		}
	}
	details := []ArgDetail{}
	for _, description := range base.DescribeArguments(op, args.Function) {
		suggestions := op.GetArgSuggestions(args.Function, description.Name, otherArgs)
		if suggestions == nil {
			suggestions = map[string]string{}
		}
		details = append(details, ArgDetail{ArgumentDescription: description, Suggestions: suggestions})
	}
	return base.MakeObjectOutput(details)
}

// operatorNameSuggestions returns the operator names as a map, as expected by the suggestion functions.
func operatorNameSuggestions(m *OpFlowBuilder) map[string]string {
	r := map[string]string{}
	for _, opName := range m.GE.GetOperators() {
		r[opName] = opName
	}
	return r
}

// functionNameSuggestions returns the function names of the given operator as a map.
// An unknown or empty operator results in an empty map, so that the suggestions of the
// dependent arguments stay empty until a valid operator is selected.
func functionNameSuggestions(m *OpFlowBuilder, operator string) map[string]string {
	r := map[string]string{}
	if operator == "" {
		return r
	}
	op := m.GE.GetOperator(operator)
	if op == nil {
		return r
	}
	for _, fn := range op.GetFunctions() {
		r[fn] = fn
	}
	return r
}
