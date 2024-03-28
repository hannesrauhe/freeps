package telegram

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type TelegramConfig struct {
	Enabled            bool
	Token              string
	AllowedUsers       []string
	DebugMessages      bool
	StoreChatNamespace string
}

type TelegramCallbackResponse struct {
	T string `json:",omitempty"` // TemplateActionID
	F bool   `json:",omitempty"` // Finished ?
	P int    `json:",omitempty"` // processed Args
	C string `json:",omitempty"` // last choice
	K bool   `json:",omitempty"` // request to type value instead of choosing
}

type ButtonWrapper struct {
	Button tgbotapi.InlineKeyboardButton
	Choice string
}

func (m *OpTelegram) newJSONButton(name string, tcr *TelegramCallbackResponse) *ButtonWrapper {
	if len(name) > 15 {
		name = name[0:15]
	}
	byt, err := json.Marshal(tcr)
	if err != nil {
		panic(err)
	}
	s := string(byt)
	return &ButtonWrapper{Button: tgbotapi.NewInlineKeyboardButtonData(name, s), Choice: tcr.C}
}

func (m *OpTelegram) getReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{}
	row := []tgbotapi.KeyboardButton{}
	counter := 0
	for _, k := range m.GE.GetOperators() {
		row = append(row, tgbotapi.NewKeyboardButton(k))
		counter++
		if counter != 0 && counter%3 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return tgbotapi.NewOneTimeReplyKeyboard(rows...)
}

func (m *OpTelegram) getCurrentOp(graphName string) (base.FreepsBaseOperator, *freepsgraph.GraphDesc) {
	graph, err := freepsstore.GetGraph(graphName)
	if err != nil {
		return nil, nil
	}
	if graph.Operations == nil || len(graph.Operations) == 0 {
		return nil, nil
	}
	op := m.GE.GetOperator(graph.Operations[0].Operator)
	if op == nil {
		return nil, nil
	}
	return op, &graph
}

func (m *OpTelegram) getModButtons() []*ButtonWrapper {
	keys := make([]*ButtonWrapper, 0, len(m.GE.GetOperators()))
	for _, k := range m.GE.GetOperators() {
		tcr := TelegramCallbackResponse{F: false, P: -1, C: k}
		keys = append(keys, m.newJSONButton(k, &tcr))
	}
	return keys
}

func (m *OpTelegram) getFnButtons(tcr *TelegramCallbackResponse) []*ButtonWrapper {
	op, _ := m.getCurrentOp(tcr.T)
	if op == nil {
		return make([]*ButtonWrapper, 0)
	}
	fn := op.GetFunctions()
	keys := make([]*ButtonWrapper, 0, len(fn))
	tcr.K = true
	keys = append(keys, m.newJSONButton("<CUSTOM>", tcr))
	tcr.K = false
	for _, k := range fn {
		tcr.C = k
		keys = append(keys, m.newJSONButton(k, tcr))
	}
	return keys
}

func (m *OpTelegram) getArgsButtons(arg string, tcr *TelegramCallbackResponse) []*ButtonWrapper {
	op, gd := m.getCurrentOp(tcr.T)
	ta := gd.Operations[0]
	if op == nil {
		return make([]*ButtonWrapper, 0)
	}
	argOptions := op.GetArgSuggestions(ta.Function, arg, ta.Arguments)
	keys := make([]*ButtonWrapper, 0, len(argOptions)+2)
	tcr.F = true
	keys = append(keys, m.newJSONButton("<Execute>", tcr))
	tcr.F = false
	keys = append(keys, m.newJSONButton("<Skip "+arg+">", tcr))
	tcr.K = true
	keys = append(keys, m.newJSONButton("<CUSTOM>", tcr))
	tcr.K = false
	for k, v := range argOptions {
		tcr.C = v
		keys = append(keys, m.newJSONButton(k, tcr))
	}
	return keys
}

func (m *OpTelegram) multiChoiceKeyboard(buttons []*ButtonWrapper) (tgbotapi.InlineKeyboardMarkup, string) {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	row := []tgbotapi.InlineKeyboardButton{}
	counter := 0
	addVals := ""
	for _, b := range buttons {
		if len(*b.Button.CallbackData) > 60 {
			addVals += " " + b.Choice
			continue
		}
		row = append(row, b.Button)
		counter++
		if counter != 0 && counter%3 == 0 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...), addVals
}

func (m *OpTelegram) getModKeyboard() (tgbotapi.InlineKeyboardMarkup, string) {
	return m.multiChoiceKeyboard(m.getModButtons())
}

func (m *OpTelegram) getFnKeyboard(tcr *TelegramCallbackResponse) (tgbotapi.InlineKeyboardMarkup, string) {
	return m.multiChoiceKeyboard(m.getFnButtons(tcr))
}

func (m *OpTelegram) getArgsKeyboard(arg string, tcr *TelegramCallbackResponse) (tgbotapi.InlineKeyboardMarkup, string) {
	return m.multiChoiceKeyboard(m.getArgsButtons(arg, tcr))
}

func (m *OpTelegram) sendMessage(msg *tgbotapi.MessageConfig) {
	if m.lastMessage > 0 {
		d := tgbotapi.NewDeleteMessage(msg.ChatID, m.lastMessage)
		m.bot.Send(d)
		m.lastMessage = 0
	}
	mg, err := m.bot.Send(*msg)
	if err != nil {
		log.Println(err)
		return
	}
	m.lastMessage = mg.MessageID
}

func (m *OpTelegram) sendStartMessage(msg *tgbotapi.MessageConfig) {
	freepsstore.DeleteGraph(fmt.Sprint(msg.ChatID))
	msg.ReplyMarkup, _ = m.getModKeyboard()
	m.sendMessage(msg)
}

func (m *OpTelegram) respond(chat *tgbotapi.Chat, callbackData string, inputText string) {
	telelogger := log.WithField("component", "telegram").WithField("chat", chat.ID)
	ctx := base.NewContext(telelogger)

	telelogger.Debugf("Received message from %v: %v", chat.UserName, inputText)
	msg := tgbotapi.NewMessage(chat.ID, "Hello "+chat.FirstName+".")
	allowed := false
	for _, v := range m.tgc.AllowedUsers {
		if v == chat.UserName {
			allowed = true
			break
		}
	}
	if !allowed {
		msg.Text += " I'm not allowed to talk to you."
		if _, err := m.bot.Send(msg); err != nil {
			ctx.GetLogger().Error(err)
		}
		return
	}

	tcr, ok := m.getChatState(ctx, *chat)
	if !ok {
		tcr = TelegramCallbackResponse{}
		if callbackData != "" {
			// a button on the InlineKeyboard was pressed
			byt := []byte(callbackData)
			err := json.Unmarshal(byt, &tcr)
			if err != nil {
				msg.Text = err.Error()
				m.sendStartMessage(&msg)
				return
			}
		} else {
			// inputText contains the mod to use
			freepsstore.DeleteGraph(fmt.Sprint(chat.ID))
			tcr.P = -1
			tcr.C = inputText
		}
	} else {
		// the user was asked to provide input
		tcr.C = inputText
		m.resetChatState(ctx, *chat)
	}
	tcr.T = fmt.Sprint(chat.ID)
	op, gd := m.getCurrentOp(tcr.T)
	if op == nil {
		if !m.GE.HasOperator(tcr.C) {
			msg.Text += " Please pick an Operator"
			m.sendStartMessage(&msg)
			return
		}
		tpl := freepsgraph.GraphDesc{Operations: []freepsgraph.GraphOperationDesc{{Operator: tcr.C, Arguments: map[string]string{}, UseMainArgs: true}}, Source: "telegram"}
		freepsstore.StoreGraph(tcr.T, tpl, ctx.GetID())
		op, gd = m.getCurrentOp(tcr.T)
		msg.Text = "Pick a function for " + gd.Operations[0].Operator
		msg.ReplyMarkup, _ = m.getFnKeyboard(&tcr)
	} else if len(gd.Operations[0].Function) == 0 {
		if tcr.K {
			msg.Text = "Type a function for " + gd.Operations[0].Operator
			tcr.K = false
			m.setChatState(ctx, *chat, tcr)
		} else {
			gd.Operations[0].Function = tcr.C
			freepsstore.StoreGraph(tcr.T, *gd, ctx.GetID())
		}
	}

	if len(gd.Operations[0].Function) > 0 && !tcr.F {
		args := op.GetPossibleArgs(gd.Operations[0].Function)
		if tcr.K {
			msg.Text = fmt.Sprintf("Type a Value for %s (%s/%s)", args[tcr.P], gd.Operations[0].Operator, gd.Operations[0].Function)
			tcr.K = false
			m.setChatState(ctx, *chat, tcr)
		} else {
			if tcr.P >= 0 {
				if gd.Operations[0].Arguments == nil {
					gd.Operations[0].Arguments = make(map[string]string)
				}
				gd.Operations[0].Arguments[args[tcr.P]] = tcr.C
				freepsstore.StoreGraph(tcr.T, *gd, ctx.GetID())
			}
			tcr.C = ""
			tcr.P++
			if tcr.P >= len(args) {
				tcr.F = true
				freepsstore.StoreGraph(tcr.T, *gd, ctx.GetID())
			} else {
				addVals := ""
				msg.Text = fmt.Sprintf("Pick a Value for %s (%s/%s)", args[tcr.P], gd.Operations[0].Operator, gd.Operations[0].Function)
				msg.ReplyMarkup, addVals = m.getArgsKeyboard(args[tcr.P], &tcr)
				if len(addVals) > 0 {
					// do not use SendMessage, because that message gets deleted.... yeah, I need to clean this up
					m.bot.Send(tgbotapi.NewMessage(chat.ID, "More values:"+addVals+"."))
				}
			}
		}
	}

	if tcr.F {
		gd, err := freepsstore.GetGraph(tcr.T)
		if err != nil {
			msg.Text = err.Error()
		}
		m.resetChatState(ctx, *chat) // links the context ID of this chat to the execution of the graph
		io := m.GE.ExecuteAdHocGraph(ctx, "telegram/"+tcr.T, gd, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
		if io.IsError() {
			msg.Text = fmt.Sprintf("Error when executing operation: %v", io.GetError())
		} else if utils.StringStartsWith(io.ContentType, "image") {
			byt, err := io.GetBytes()
			if err != nil {
				msg.Text = fmt.Sprintf("Error when decoding output of operation: %v", err)
			} else {
				msg.Text = "Here is a picture for you"
				mg := tgbotapi.NewPhoto(chat.ID, tgbotapi.FileBytes{Name: "picture." + io.ContentType[6:], Bytes: byt})
				if _, err := m.bot.Send(mg); err != nil {
					telelogger.Error(err)
				}
			}
		} else {
			msg.Text = io.GetString()
			if msg.Text == "" {
				msg.Text = "Empty Result, HTTP code:" + fmt.Sprint(io.GetStatusCode())
			}
		}
		freepsstore.DeleteGraph(tcr.T)
		msg.ReplyMarkup = m.getReplyKeyboard()
	}
	m.sendMessage(&msg)
}

func (m *OpTelegram) mainLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := m.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			m.respond(update.CallbackQuery.Message.Chat, update.CallbackQuery.Data, "")
			continue
		}
		if update.Message == nil { // ignore any non-Message updates
			continue
		}
		m.respond(update.Message.Chat, "", update.Message.Text)
	}
	log.Print("Telegram Main Loop stopped")
	m.closeChan <- 1
}
