//go:build !notelegram

package telegram

import (
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/base"
)

func (m *OpTelegram) executeTrigger(ctx *base.Context, message tgbotapi.Message) *base.OperatorIO {
	tags := []string{"telegram"}
	input := base.MakePlainOutput(message.Text)
	args := base.MakeEmptyFunctionArguments()
	// freepsstore.GetGlobalStore().GetNamespaceNoError("_mqtt").SetValue(message.Topic(), input, ctx)

	out := m.GE.ExecuteFlowByTags(ctx, tags, args, input)
	return out
}

// FlowID auggestions returns suggestions for flow names
func (o *OpTelegram) FlowIDSuggestions() map[string]string {
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

type TelegramTrigger struct {
	FlowID string
}

func (m *OpTelegram) SetTopicTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TelegramTrigger) *base.OperatorIO {
	gd, found := m.GE.GetFlowDesc(args.FlowID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find flow: %v", args.FlowID)
	}
	gd.AddTags("telegram")
	err := m.GE.AddFlow(ctx, args.FlowID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify flow: %v", err)
	}

	return base.MakeEmptyOutput()
}
