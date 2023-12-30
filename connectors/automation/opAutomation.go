package automation

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type Rule struct {
	Name         string
	Trigger      base.FreepsTrigger
	TriggerValue string
	Graph        string
}

// OpAutomation is the operator for automation
type OpAutomation struct {
	CR      *utils.ConfigReader
	GE      *freepsgraph.GraphEngine
	ruleMap utils.CIMap[Rule]
}

func (oa *OpAutomation) getRulesForTrigger(opName string, triggers []base.FreepsTrigger) []Rule {
	ret := []Rule{}
	for _, trigger := range triggers {
		triggerName := trigger.GetName()
		gm := oa.GE.GetGraphDescByTag([]string{opName, triggerName})
		for graphName, graphDesc := range gm {
			for _, tag := range graphDesc.Tags {
				triggerTag, triggerValue := freepsgraph.SplitTag(tag)
				if utils.StringCmpIgnoreCase(triggerTag, triggerName) {
					r := Rule{Trigger: trigger, Graph: graphName, TriggerValue: triggerValue}
					ret = append(ret, r)
				}
			}
		}
	}
	return ret
}

func (oa *OpAutomation) buildRuleMap() {
	tMap := make(map[string][]Rule)
	for _, op := range oa.GE.GetOperators() {
		opInstance := oa.GE.GetOperator(op)
		opTriggers := opInstance.GetTriggers()
		if len(opTriggers) == 0 {
			continue
		}
		tMap[opInstance.GetName()] = oa.getRulesForTrigger(opInstance.GetName(), opTriggers)
	}
	oa.ruleMap = utils.NewCIMapFromValues(tMap, Rule{})
}

var _ base.FreepsOperator = &OpAutomation{}

type GetTriggerArgs struct {
	Operator string
	Trigger  *string
}

func (gta *GetTriggerArgs) OperatorSuggestions(oa *OpAutomation) []string {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}
	return oa.ruleMap.GetKeys()
}

func (gta *GetTriggerArgs) TriggerSuggestions(oa *OpAutomation) []string {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}

	ret := []string{}
	if gta.Operator != "" {
		for _, r := range oa.ruleMap.GetArray(gta.Operator) {
			ret = append(ret, r.Trigger.GetName())
		}
	}

	for _, rule := range oa.ruleMap.GetOriginalCaseMap() {
		if len(rule) > 0 {
			ret = append(ret, rule[0].Trigger.GetName())
		}
	}
	return ret
}

// GetTriggerOptions returns a list of all triggers
func (oa *OpAutomation) GetTriggerOptions(ctx *base.Context, mainInput *base.OperatorIO, args GetTriggerArgs) *base.OperatorIO {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}

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

// GetActiveAutomation returns a list of triggers and graphs that are triggered by them
func (oa *OpAutomation) GetActiveAutomation(ctx *base.Context) *base.OperatorIO {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}

	return base.MakeObjectOutput(oa.ruleMap)
}

type CreateRuleArgs struct {
	Operator     string
	Trigger      string
	TriggerValue string
	Graph        string
}

func (gta *CreateRuleArgs) OperatorSuggestions(oa *OpAutomation) []string {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}
	return oa.ruleMap.GetKeys()
}

func (gta *CreateRuleArgs) TriggerSuggestions(oa *OpAutomation) []string {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}

	ret := []string{}
	if gta.Operator != "" {
		for _, r := range oa.ruleMap.GetArray(gta.Operator) {
			ret = append(ret, r.Trigger.GetName())
		}
	}

	for _, rule := range oa.ruleMap.GetOriginalCaseMap() {
		if len(rule) > 0 {
			ret = append(ret, rule[0].Trigger.GetName())
		}
	}
	return ret
}

func (gta *CreateRuleArgs) GraphSuggestions(oa *OpAutomation) []string {
	ret := []string{}
	for n := range oa.GE.GetAllGraphDesc() {
		ret = append(ret, n)
	}
	return ret
}

// CreateRule adds tags to a graph so this graph is executed when the given trigger triggers
func (oa *OpAutomation) CreateRule(ctx *base.Context, mainInput *base.OperatorIO, args CreateRuleArgs) *base.OperatorIO {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}

	gd, exists := oa.GE.GetGraphDesc(args.Graph)
	if !exists {
		return base.MakeOutputError(http.StatusBadRequest, "Graph \"%v\" does not exist", args.Graph)
	}
	opTag := utils.StringToLower(args.Operator)
	triggerTag := fmt.Sprintf("%v:%v", utils.StringToLower(args.Trigger), args.TriggerValue)
	gd.AddTags(opTag, triggerTag)
	oa.GE.AddGraph(args.Graph, *gd, true)

	return base.MakeEmptyOutput()
}

// GetRules
func (oa *OpAutomation) GetRules(ctx *base.Context) *base.OperatorIO {
	if oa.ruleMap == nil {
		oa.buildRuleMap()
	}

	return base.MakeObjectOutput(oa.ruleMap.GetOriginalCaseMap())
}
