package opalert

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
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

// SetSeverityTrigger
func (oc *OpAlert) SetSeverityTrigger(ctx *base.Context, mainInput *base.OperatorIO, args SeverityTrigger) *base.OperatorIO {
	tags := make([]string, args.Severity)
	for i := 1; i <= args.Severity; i++ {
		tags[i-1] = fmt.Sprintf("severity:%v", i)
	}

	return oc.setTrigger(ctx, args.GraphID, tags...)
}

func (oc *OpAlert) execTriggers(causedByCtx *base.Context, alert AlertWithMetadata) {
	triggerTags := []string{fmt.Sprintf("severity:%v", alert.Severity)}
	tagGroups := [][]string{{"alert"}, triggerTags}
	args, err := utils.ObjectToArgsMap(alert)
	if err == nil {
		desc := fmt.Sprintf("Cannot parse alert: %v", err)
		category := "alertOp"
		// disable triggers so we do not run into endless loops:
		oc.SetAlert(causedByCtx, base.MakeEmptyOutput(), Alert{Name: "AlertGraphTrigger", Desc: &desc, Category: &category}, base.NewFunctionArguments(map[string]string{"noTrigger": "1"}))
	}

	oc.GE.ExecuteGraphByTagsExtended(causedByCtx, tagGroups, args, base.MakeEmptyOutput())
}
