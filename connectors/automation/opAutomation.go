package automation

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// OpAutomation is the operator for automation
type OpAutomation struct {
	CR *utils.ConfigReader
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperator = &OpAutomation{}

type GetTriggerArgs struct {
	Operator string
	Trigger  *string
}

func (gta *GetTriggerArgs) OperatorSuggestions(oa *OpAutomation) []string {
	ret := []string{}
	for _, op := range oa.GE.GetOperators() {
		opInstance := oa.GE.GetOperator(op)
		if len(opInstance.GetTriggers()) > 0 {
			ret = append(ret, op)
		}
	}
	return ret
}

func (gta *GetTriggerArgs) TriggerSuggestions(oa *OpAutomation) []string {
	ret := []string{}
	opInstance := oa.GE.GetOperator(gta.Operator)
	if opInstance == nil {
		return ret
	}
	for _, trigger := range opInstance.GetTriggers() {
		ret = append(ret, trigger.GetName())
	}
	return ret
}

// GetTrigers returns a list of all triggers
func (oa *OpAutomation) GetTriggerOptions(ctx *base.Context, mainInput *base.OperatorIO, args GetTriggerArgs) *base.OperatorIO {
	ret := map[string][]string{}
	opInstance := oa.GE.GetOperator(args.Operator)
	if opInstance == nil {
		return base.MakeOutputError(404, "Operator %v not found", args.Operator)
	}
	for _, trigger := range opInstance.GetTriggers() {
		if args.Trigger == nil || utils.StringCmpIgnoreCase(trigger.GetName(), *args.Trigger) {
			ret[utils.StringToLower(trigger.GetName())] = trigger.GetSuggestions()
		}
	}
	return base.MakeObjectOutput(ret)
}
