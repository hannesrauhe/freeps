package smtp

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
)

// FlowID auggestions returns suggestions for flow names
func (sm *OpSMTP) FlowIDSuggestions() map[string]string {
	flowNames := map[string]string{}
	res := sm.GE.GetAllFlowDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, sm.GE)
		_, exists := flowNames[info.DisplayName]
		if !exists {
			flowNames[info.DisplayName] = id
		} else {
			flowNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return flowNames
}

type SenderTrigger struct {
	FlowID string
	Sender string
}

// TopicSuggestions returns known topics
func (tt *SenderTrigger) TopicSuggestions(otherArgs base.FunctionArguments, o *OpSMTP) []string {
	maxSize := 20
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_mqtt")
	if err != nil {
		return []string{}
	}
	allKeys := ns.GetKeys()
	if len(allKeys) <= maxSize {
		return allKeys
	}
	filteredKeys := allKeys
	if tt.Sender != "" {
		filteredKeys = []string{}
		for _, k := range allKeys {
			if utils.StringStartsWith(k, tt.Sender) {
				filteredKeys = append(filteredKeys, k)
				if len(filteredKeys) >= maxSize {
					break
				}
			}
		}
	}
	if len(filteredKeys) <= maxSize {
		return filteredKeys
	}

	h1Keys := []string{}
	lastPrefix := ""
	for _, k := range filteredKeys {
		if lastPrefix != "" && utils.StringStartsWith(k, lastPrefix) {
			continue
		}
		parts := strings.SplitN(k, "/", 2)
		lastPrefix = parts[0]
		h1Keys = append(h1Keys, lastPrefix)
		if len(h1Keys) >= maxSize {
			break
		}
	}
	return h1Keys
}

func (sm *OpSMTP) SetSenderTrigger(ctx *base.Context, mainInput *base.OperatorIO, args SenderTrigger) *base.OperatorIO {
	gd, found := sm.GE.GetFlowDesc(args.FlowID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find flow: %v", args.FlowID)
	}
	gd.AddTags("smtp", "sender:"+args.Sender)
	err := sm.GE.AddFlow(ctx, args.FlowID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify flow: %v", err)
	}

	return base.MakeEmptyOutput()
}
