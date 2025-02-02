package freepsflow

import (
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/base"
)

type OpFlow struct {
	ge *FlowEngine
}

type OpFlowByTag struct {
	ge *FlowEngine
}

var _ base.FreepsBaseOperator = &OpFlow{}
var _ base.FreepsBaseOperator = &OpFlowByTag{}

func (o *OpFlow) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	if input.IsError() { // flow has been called by another operator, but the operator returned an error
		return input
	}
	return o.ge.ExecuteFlow(ctx, fn, fa, input)
}

func (o *OpFlow) ExecuteOld(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	if input.IsError() { // flow has been called by another operator, but the operator returned an error
		return input
	}
	return o.ge.ExecuteFlow(ctx, fn, base.NewFunctionArguments(args), input)
}

// GetName returns the name of the operator
func (o *OpFlow) GetName() string {
	return "flow"
}

// GetFunctions returns a list of flows stored in the engine
func (o *OpFlow) GetFunctions() []string {
	agd := o.ge.GetAllFlowDesc()
	flows := make([]string, 0, len(agd))
	for n := range agd {
		flows = append(flows, n)
	}
	sort.Strings(flows)
	return flows
}

// GetPossibleArgs returns suggestions based on the suggestions of the operators in the flow
func (o *OpFlow) GetPossibleArgs(fn string) []string {
	agd, exists := o.ge.GetFlowDesc(fn)
	if !exists {
		return []string{}
	}
	possibleArgs := make([]string, 0)
	for _, op := range agd.Operations {
		if !op.UseMainArgs {
			continue
		}
		thisOpArgs := o.ge.GetOperator(op.Operator).GetPossibleArgs(op.Function)
		possibleArgs = append(possibleArgs, thisOpArgs...)
	}
	return possibleArgs
}

// GetArgSuggestions returns suggestions based on the suggestions of the operators in the flow
func (o *OpFlow) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	agd, exists := o.ge.GetFlowDesc(fn)
	if !exists {
		return map[string]string{}
	}
	possibleValues := make(map[string]string, 0)
	for _, op := range agd.Operations {
		if !op.UseMainArgs {
			continue
		}
		// build a map of all arguments that will be passed to this operation on execution
		thisOpArgs := make(map[string]string)
		for k, v := range op.Arguments {
			thisOpArgs[k] = v
		}
		for k, v := range otherArgs.GetOriginalCaseMapOnlyFirst() {
			thisOpArgs[k] = v
		}
		thisOpSuggestions := o.ge.GetOperator(op.Operator).GetArgSuggestions(op.Function, arg, base.NewFunctionArguments(thisOpArgs))
		for k, v := range thisOpSuggestions {
			possibleValues[k] = v
		}
	}
	return possibleValues
}

// StartListening (noOp)
func (o *OpFlow) StartListening(*base.Context) {
}

// Shutdown (noOp)
func (o *OpFlow) Shutdown(*base.Context) {
}

// GetHook returns the hook for this operator
func (o *OpFlow) GetHook() interface{} {
	return nil
}

/*** By Tag ****/

func (o *OpFlowByTag) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return o.ExecuteOld(ctx, fn, fa.GetOriginalCaseMapJoined(), input)
}

func (o *OpFlowByTag) ExecuteOld(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	if input.IsError() { // flow has been called by another operator, but the operator returned an error
		return input
	}
	tags := []string{}
	if fn != "" {
		tags = append(tags, fn)
	}
	addTstr := args["additionalTags"]
	if addTstr != "" {
		tags = append(tags, strings.Split(addTstr, ",")...)
	}

	return o.ge.ExecuteFlowByTags(ctx, tags, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
}

// GetName returns the name of the operator
func (o *OpFlowByTag) GetName() string {
	return "flowbytag"
}

// GetFunctions returns a list of all available tags
func (o *OpFlowByTag) GetFunctions() []string {
	agd := o.ge.GetTags()
	flows := make([]string, 0, len(agd))
	for n := range agd {
		flows = append(flows, n)
	}
	sort.Strings(flows)
	return flows
}

// GetPossibleArgs returns the additonalTags Option
func (o *OpFlowByTag) GetPossibleArgs(fn string) []string {
	return []string{"additionalTags"}
}

// GetArgSuggestions returns addtional tags
func (o *OpFlowByTag) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	return o.ge.GetTags()
}

// StartListening (noOp)
func (o *OpFlowByTag) StartListening(*base.Context) {
}

// Shutdown (noOp)
func (o *OpFlowByTag) Shutdown(*base.Context) {
}

// GetHook returns the hook for this operator
func (o *OpFlowByTag) GetHook() interface{} {
	return nil
}
