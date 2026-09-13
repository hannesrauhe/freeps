package flowbuilder

import (
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// This file exposes the operator metadata that drives the flow editor UI as a
// read-only JSON API. The metadata forms a four level ladder, one endpoint per level:
//
//	operators -> functions -> arguments -> argument details
//
// so a client can walk from the top down without ever fetching more than it asked for.
// Every level returns objects with a Name and a human-readable Description: for arguments
// it comes from the "doc" struct tag, for operators and functions from the doc comments,
// harvested into operatorDescriptions_generated.go by "make generate". A missing entry
// simply results in an empty description.

// OperatorDetail is one operator with a human-readable description.
// The description comes from the doc comment of the operator type, see
// operatorDescriptions_generated.go.
type OperatorDetail struct {
	Name        string
	Description string
}

// FunctionDetail is one function of an operator with a human-readable description.
// The description comes from the doc comment of the method, see
// operatorDescriptions_generated.go.
type FunctionDetail struct {
	Name        string
	Description string
}

// ListOperatorsArgs are the arguments for ListOperators (there are none).
type ListOperatorsArgs struct{}

// ListOperators returns name and description of all operators that are currently registered
// in the flow engine.
func (m *OpFlowBuilder) ListOperators(ctx *base.Context, input *base.OperatorIO, args ListOperatorsArgs) *base.OperatorIO {
	details := []OperatorDetail{}
	for _, name := range m.GE.GetOperators() {
		details = append(details, OperatorDetail{Name: name, Description: operatorDescriptions[baseOperatorName(name)]})
	}
	return base.MakeObjectOutput(details)
}

// ListFunctionsArgs are the arguments for ListFunctions.
type ListFunctionsArgs struct {
	Operator string
}

// OperatorSuggestions returns suggestions for the operator name.
func (arg *ListFunctionsArgs) OperatorSuggestions(otherArgs base.FunctionArguments, m *OpFlowBuilder) map[string]string {
	return operatorNameSuggestions(m)
}

// ListFunctions returns name and description of all functions of the given operator, sorted
// alphabetically. Operators are free to order their functions as they like (exec puts "run"
// first), so the stable order is created here rather than in GetFunctions.
func (m *OpFlowBuilder) ListFunctions(ctx *base.Context, input *base.OperatorIO, args ListFunctionsArgs) *base.OperatorIO {
	op := m.GE.GetOperator(args.Operator)
	if op == nil {
		return base.MakeOutputError(404, "Operator not found: %v", args.Operator)
	}
	details := []FunctionDetail{}
	descriptions := functionDescriptions[baseOperatorName(args.Operator)]
	for _, name := range op.GetFunctions() {
		details = append(details, FunctionDetail{Name: name, Description: descriptions[name]})
	}
	sort.Slice(details, func(i, j int) bool {
		return strings.ToLower(details[i].Name) < strings.ToLower(details[j].Name)
	})
	return base.MakeObjectOutput(details)
}

// baseOperatorName returns the name of the operator type behind a registered name. Config
// variations are registered under their config section name ("http.internal"), so the part
// before the first dot is the name the generated descriptions are keyed by.
func baseOperatorName(name string) string {
	if baseName, _, found := strings.Cut(name, "."); found {
		return baseName
	}
	return utils.StringToLower(name)
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

// OperatorArgs returns name, type, requiredness and description of all arguments of the
// given function of the given operator. This is the same list as ArgDetails, without the
// value suggestions.
func (m *OpFlowBuilder) OperatorArgs(ctx *base.Context, input *base.OperatorIO, args OperatorArgsArgs) *base.OperatorIO {
	op := m.GE.GetOperator(args.Operator)
	if op == nil {
		return base.MakeOutputError(404, "Operator not found: %v", args.Operator)
	}
	return base.MakeObjectOutput(op.GetArgumentDescriptions(args.Function))
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
	for _, description := range op.GetArgumentDescriptions(args.Function) {
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
