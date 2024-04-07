package fritz

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

func (m *OpFritz) executeTrigger(ctx *base.Context, host Host, addTags ...string) *base.OperatorIO {
	tags := []string{"fritz"}
	tags = append(tags, addTags...)
	input := base.MakeObjectOutput(host)
	args, _ := base.NewFunctionArgumentsFromObject(host)

	out := m.GE.ExecuteGraphByTags(ctx, tags, args, input)
	return out
}

func (m *OpFritz) setTrigger(ctx *base.Context, graphID string, addTags ...string) *base.OperatorIO {
	gd, found := m.GE.GetGraphDesc(graphID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", graphID)
	}
	gd.AddTags("fritz")
	gd.AddTags(addTags...)
	err := m.GE.AddGraph(ctx, graphID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

// GraphID auggestions returns suggestions for graph names
func (m *OpFritz) GraphIDSuggestions() map[string]string {
	graphNames := map[string]string{}
	res := m.GE.GetAllGraphDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, m.GE)
		_, exists := graphNames[info.DisplayName]
		if !exists {
			graphNames[info.DisplayName] = id
		} else {
			graphNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return graphNames
}

func (h *HostTrigger) MACAddressSuggestions(m *OpFritz) map[string]string {
	res := m.getHostsNamespace().GetSearchResultWithMetadata("", h.MACAddress, "", time.Duration(0), time.Duration(math.MaxInt64))
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

func (m *OpFritz) SetHostActiveTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	return m.setTrigger(ctx, args.GraphID, "active:"+args.MACAddress)
}

func (m *OpFritz) SetHostInactiveTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	return m.setTrigger(ctx, args.GraphID, "inactive:"+args.MACAddress)
}
