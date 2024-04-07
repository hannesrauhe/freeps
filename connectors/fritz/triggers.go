package fritz

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

func (o *OpFritz) executeTrigger(ctx *base.Context, host Host, addTags ...string) *base.OperatorIO {
	tags := []string{"fritz"}
	tags = append(tags, addTags...)
	input := base.MakeObjectOutput(host)
	args, _ := base.NewFunctionArgumentsFromObject(host)

	out := o.GE.ExecuteGraphByTags(ctx, tags, args, input)
	return out
}

func (o *OpFritz) setTrigger(ctx *base.Context, graphID string, addTags ...string) *base.OperatorIO {
	gd, found := o.GE.GetGraphDesc(graphID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", graphID)
	}
	gd.AddTags("fritz")
	gd.AddTags(addTags...)
	err := o.GE.AddGraph(ctx, graphID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

// GraphID auggestions returns suggestions for graph names
func (o *OpFritz) GraphIDSuggestions() map[string]string {
	graphNames := map[string]string{}
	res := o.GE.GetAllGraphDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, o.GE)
		_, exists := graphNames[info.DisplayName]
		if !exists {
			graphNames[info.DisplayName] = id
		} else {
			graphNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return graphNames
}

func (h *HostTrigger) MACAddressSuggestions(o *OpFritz) map[string]string {
	res := o.getHostsNamespace().GetSearchResultWithMetadata("", h.MACAddress, "", time.Duration(0), time.Duration(math.MaxInt64))
	macs := map[string]string{}
	for mac, hEntry := range res {
		if utils.StringStartsWith(mac, "IP:") {
			continue
		}
		var h Host
		err := hEntry.ParseJSON(&h)
		if err != nil {
			continue
		}
		macs[fmt.Sprintf("%v (Mac: %v)", h.HostName, mac)] = mac
	}
	return macs
}

type HostTrigger struct {
	GraphID    string
	MACAddress string
}

func (o *OpFritz) SetHostActiveTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	return o.setTrigger(ctx, args.GraphID, "active:"+args.MACAddress)
}

func (o *OpFritz) SetHostInactiveTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	return o.setTrigger(ctx, args.GraphID, "inactive:"+args.MACAddress)
}
