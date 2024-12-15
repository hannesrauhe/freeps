package opalert

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

func (oc *OpAlert) setTrigger(ctx *base.Context, graphId string, tags ...string) *base.OperatorIO {
	gd, found := oc.GE.GetGraphDesc(graphId)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", graphId)
	}

	gd.AddTags("alert")
	gd.AddTags(tags...)
	err := oc.GE.AddGraph(ctx, graphId, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

type SeverityTrigger struct {
	Severity int
	GraphID  string
}

// GraphID auggestions returns suggestions for graph names
func (oc *OpAlert) GraphIDSuggestions() map[string]string {
	graphNames := map[string]string{}
	res := oc.GE.GetAllGraphDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, oc.GE)
		_, exists := graphNames[info.DisplayName]
		if !exists {
			graphNames[info.DisplayName] = id
		} else {
			graphNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return graphNames
}

// SetSeverityTrigger
func (oc *OpAlert) SetSeverityTrigger(ctx *base.Context, mainInput *base.OperatorIO, args SeverityTrigger) *base.OperatorIO {
	tags := make([]string, args.Severity)
	for i := 1; i <= args.Severity; i++ {
		tags[i-1] = fmt.Sprintf("severity:%v", i)
	}

	return oc.setTrigger(ctx, args.GraphID, tags...)
}

type NameTrigger struct {
	Name    string
	GraphID string
}

// NameSuggestions returns suggestions for alert names
func (arg *NameTrigger) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(nil, true)
}

// SetAlertSetTrigger defines a trigger for setting an alert
func (oc *OpAlert) SetAlertSetTrigger(ctx *base.Context, mainInput *base.OperatorIO, args NameTrigger) *base.OperatorIO {
	return oc.setTrigger(ctx, args.GraphID, fmt.Sprintf("set:%v", args.Name))
}

// SetAlertResetTrigger defines a trigger for resetting an alert
func (oc *OpAlert) SetAlertResetTrigger(ctx *base.Context, mainInput *base.OperatorIO, args NameTrigger) *base.OperatorIO {
	return oc.setTrigger(ctx, args.GraphID, fmt.Sprintf("reset:%v", args.Name))
}

func (oc *OpAlert) execTriggers(causedByCtx *base.Context, alert AlertWithMetadata) {
	triggerTags := []string{}
	if !alert.IsExpired() {
		triggerTags = []string{fmt.Sprintf("severity:%v", alert.Severity), fmt.Sprintf("set:%v", alert.GetFullName())}
	} else {
		triggerTags = []string{fmt.Sprintf("reset:%v", alert.GetFullName())}
	}
	tagGroups := [][]string{{"alert"}, triggerTags}
	args, err := base.NewFunctionArgumentsFromObject(alert)
	if err != nil {
		desc := fmt.Sprintf("Cannot parse alert: %v", err)
		// disable triggers so we do not run into endless loops:
		oc.SetAlert(causedByCtx, base.MakeEmptyOutput(), Alert{Name: "AlertGraphTrigger", Desc: &desc, Category: "alertOp", Severity: 1}, base.NewFunctionArguments(map[string]string{"noTrigger": "1"}))
	}

	if alert.IsSilenced() {
		return
	}
	oc.GE.ExecuteGraphByTagsExtended(causedByCtx, tagGroups, args, base.MakeEmptyOutput())
}
