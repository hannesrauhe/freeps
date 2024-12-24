package fritz

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

func (o *OpFritz) executeTrigger(ctx *base.Context, host Host, addTags ...string) *base.OperatorIO {
	tags := []string{o.name}
	tags = append(tags, addTags...)
	input := base.MakeObjectOutput(host)
	args, _ := base.NewFunctionArgumentsFromObject(host)

	out := o.GE.ExecuteFlowByTags(ctx, tags, args, input)
	return out
}

func (o *OpFritz) setTrigger(ctx *base.Context, flowID string, addTags ...string) *base.OperatorIO {
	gd, found := o.GE.GetFlowDesc(flowID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find flow: %v", flowID)
	}
	gd.AddTags(o.name)
	gd.AddTags(addTags...)
	err := o.GE.AddFlow(ctx, flowID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify flow: %v", err)
	}

	return base.MakeEmptyOutput()
}

// FlowID auggestions returns suggestions for flow names
func (o *OpFritz) FlowIDSuggestions() map[string]string {
	flowNames := map[string]string{}
	res := o.GE.GetAllFlowDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, o.GE)
		_, exists := flowNames[info.DisplayName]
		if !exists {
			flowNames[info.DisplayName] = id
		} else {
			flowNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return flowNames
}

func (h *HostTrigger) MACAddressSuggestions(o *OpFritz) map[string]string {
	return o.getHostSuggestions(h.MACAddress)
}

type HostTrigger struct {
	FlowID     string
	MACAddress string
}

func (o *OpFritz) SetHostActiveTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	return o.setTrigger(ctx, args.FlowID, "active:"+args.MACAddress)
}

func (o *OpFritz) SetHostInactiveTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	return o.setTrigger(ctx, args.FlowID, "inactive:"+args.MACAddress)
}
