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
	args := base.MakeEmptyFunctionArguments()

	out := m.GE.ExecuteGraphByTags(ctx, tags, args, input)
	return out
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

func (m *OpFritz) SetHostTrigger(ctx *base.Context, mainInput *base.OperatorIO, args HostTrigger) *base.OperatorIO {
	gd, found := m.GE.GetGraphDesc(args.GraphID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", args.GraphID)
	}
	gd.AddTags("fritz")
	err := m.GE.AddGraph(ctx, args.GraphID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}
