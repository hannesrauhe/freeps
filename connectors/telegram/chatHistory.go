//go:build !notelegram

package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
)

func (m *OpTelegram) getRecentChats() map[string]string {
	r := make(map[string]string)
	ns := freepsstore.GetGlobalStore().GetNamespaceNoError(m.tgc.StoreChatNamespace)
	for ChatID, e := range ns.GetAllValues(10) {
		completeState := ChatState{}
		err := e.ParseJSON(&completeState)
		if err != nil {
			continue
		}
		if completeState.Chat == nil {
			continue
		}
		t := ChatID
		if completeState.Chat.Title != "" {
			t = fmt.Sprintf("%v - %v (%v)", completeState.Chat.UserName, completeState.Chat.Title, completeState.Chat.ID)
		} else {
			t = fmt.Sprintf("%v (%v)", completeState.Chat.UserName, completeState.Chat.ID)
		}
		r[t] = ChatID
	}

	return r
}

func (m *OpTelegram) getChatState(ctx *base.Context, chat tgbotapi.Chat) (TelegramCallbackResponse, bool) {
	ns := freepsstore.GetGlobalStore().GetNamespaceNoError(m.tgc.StoreChatNamespace)
	storeEntry := ns.GetValue(fmt.Sprint(chat.ID))

	if storeEntry == freepsstore.NotFoundEntry {
		ns.SetValue(fmt.Sprint(chat.ID), base.MakeObjectOutput(ChatState{Chat: &chat, CallbackResponse: nil}), ctx)
		return TelegramCallbackResponse{}, false
	}

	completeState := ChatState{}
	err := storeEntry.GetData().ParseJSON(&completeState)
	if err != nil {
		ctx.GetLogger().Errorf("Error when parsing chat state: %v", err)
		return TelegramCallbackResponse{}, false
	}

	if completeState.CallbackResponse == nil {
		return TelegramCallbackResponse{}, false
	}
	return *completeState.CallbackResponse, true
}

func (m *OpTelegram) resetChatState(ctx *base.Context, chat tgbotapi.Chat) {
	ns := freepsstore.GetGlobalStore().GetNamespaceNoError(m.tgc.StoreChatNamespace)
	completeState := ChatState{Chat: &chat, CallbackResponse: nil}
	ns.SetValue(fmt.Sprint(chat.ID), base.MakeObjectOutput(completeState), ctx)
}

func (m *OpTelegram) setChatState(ctx *base.Context, chat tgbotapi.Chat, tcr TelegramCallbackResponse) {
	ns := freepsstore.GetGlobalStore().GetNamespaceNoError(m.tgc.StoreChatNamespace)
	completeState := ChatState{Chat: &chat, CallbackResponse: &tcr}
	ns.SetValue(fmt.Sprint(chat.ID), base.MakeObjectOutput(completeState), ctx)
}
