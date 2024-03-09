package opalert

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type AlertTrigger struct {
	Severity int
	GraphID  string
}

// SetAlertTrigger
func (oc *OpAlert) SetAlertTrigger(ctx *base.Context, mainInput *base.OperatorIO, args AlertTrigger) *base.OperatorIO {
	gd, err := oc.GE.DeleteGraph(ctx, args.GraphID)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't modify graph: %v", err)
	}

	gd.AddTags("alert", fmt.Sprintf("severity:%v", args.Severity))
	oc.GE.AddGraph(ctx, args.GraphID, *gd, false)

	return base.MakeEmptyOutput()
}

func (oc *OpAlert) execAlertGraphs(causedByCtx *base.Context, alert AlertWithMetadata) {
	triggerTags := []string{fmt.Sprintf("severity:%v", alert.Severity)} //TODO(HR): +lower severities
	tagGroups := [][]string{{"alert"}, triggerTags}
	args, err := utils.ObjectToArgsMap(alert)
	if err == nil {
		// setting an alert here might cause an endless loop...
		causedByCtx.GetLogger().Errorf("Cannot trigger alert graphs: %v", err)
		// oc.SetAlert(causedByCtx, base.MakeByteOutput(), Alert{Name: "AlertGraphTrigger", Desc: fmt.Sprintf("Cannot parse alert: %v", err), },)
	}

	oc.GE.ExecuteGraphByTagsExtended(causedByCtx, tagGroups, args, base.MakeEmptyOutput())
}
