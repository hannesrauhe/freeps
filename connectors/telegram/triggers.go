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
	// freepsstore.GetGlobalStore().GetNamespaceNoError("_mqtt").SetValue(message.Topic(), input, ctx.GetID())

	out := m.GE.ExecuteGraphByTags(ctx, tags, args, input)
	return out
}

// GraphID auggestions returns suggestions for graph names
func (o *OpTelegram) GraphIDSuggestions() map[string]string {
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

type TelegramTrigger struct {
	GraphID string
}

func (m *OpTelegram) SetTopicTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TelegramTrigger) *base.OperatorIO {
	gd, found := m.GE.GetGraphDesc(args.GraphID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", args.GraphID)
	}
	gd.AddTags("telegram")
	err := m.GE.AddGraph(ctx, args.GraphID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}
