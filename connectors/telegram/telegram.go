package telegram

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type TelegramConfig struct {
	Token         string
	AllowedUsers  []string
	DebugMessages bool
}

var DefaultTelegramConfig = TelegramConfig{Token: ""}

type Telegraminator struct {
	ge          *freepsgraph.GraphEngine
	bot         *tgbotapi.BotAPI
	tgc         *TelegramConfig
	lastMessage int
	chatState   map[int64]TelegramCallbackResponse
	closeChan   chan int
}

type TelegramCallbackResponse struct {
	T string `json:",omitempty"` // TemplateActionID
	F bool   `json:",omitempty"` // Finished ?
	P int    `json:",omitempty"` // processed Args
	C string `json:",omitempty"` // last choice
	K bool   `json:",omitempty"` // request to type value instead of choosing
}

func (r *Telegraminator) Shutdown(ctx context.Context) {
	if r.bot != nil {
		r.bot.StopReceivingUpdates()
		<-r.closeChan
		r.bot = nil
	}
}

type ButtonWrapper struct {
	Button tgbotapi.InlineKeyboardButton
	Choice string
}

func (r *Telegraminator) newJSONButton(name string, tcr *TelegramCallbackResponse) *ButtonWrapper {
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

func (r *Telegraminator) getReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{}
	row := []tgbotapi.KeyboardButton{}
	counter := 0
	for _, k := range r.ge.GetOperators() {
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

func (r *Telegraminator) getCurrentOp(graphName string) (freepsgraph.FreepsOperator, *freepsgraph.GraphOperationDesc) {
	graph, exists := r.ge.GetGraphDesc(graphName)
	if !exists {
		return nil, nil
	}
	if graph.Operations == nil || len(graph.Operations) == 0 {
		return nil, nil
	}
	op := r.ge.GetOperator(graph.Operations[0].Operator)
	if op == nil {
		return nil, nil
	}
	return op, &graph.Operations[0]
}

func (r *Telegraminator) getModButtons() []*ButtonWrapper {
	keys := make([]*ButtonWrapper, 0, len(r.ge.GetOperators()))
	for _, k := range r.ge.GetOperators() {
		tcr := TelegramCallbackResponse{F: false, P: -1, C: k}
		keys = append(keys, r.newJSONButton(k, &tcr))
	}
	return keys
}

func (r *Telegraminator) getFnButtons(tcr *TelegramCallbackResponse) []*ButtonWrapper {
	op, _ := r.getCurrentOp(tcr.T)
	if op == nil {
		return make([]*ButtonWrapper, 0)
	}
	fn := op.GetFunctions()
	keys := make([]*ButtonWrapper, 0, len(fn))
	tcr.K = true
	keys = append(keys, r.newJSONButton("<CUSTOM>", tcr))
	tcr.K = false
	for _, k := range fn {
		tcr.C = k
		keys = append(keys, r.newJSONButton(k, tcr))
	}
	return keys
}

func (r *Telegraminator) getArgsButtons(arg string, tcr *TelegramCallbackResponse) []*ButtonWrapper {
	op, ta := r.getCurrentOp(tcr.T)
	if op == nil {
		return make([]*ButtonWrapper, 0)
	}
	argOptions := op.GetArgSuggestions(ta.Function, arg, ta.Arguments)
	keys := make([]*ButtonWrapper, 0, len(argOptions)+2)
	tcr.F = true
	keys = append(keys, r.newJSONButton("<Execute>", tcr))
	tcr.F = false
	keys = append(keys, r.newJSONButton("<Skip "+arg+">", tcr))
	tcr.K = true
	keys = append(keys, r.newJSONButton("<CUSTOM>", tcr))
	tcr.K = false
	for k, v := range argOptions {
		tcr.C = v
		keys = append(keys, r.newJSONButton(k, tcr))
	}
	return keys
}

func (r *Telegraminator) multiChoiceKeyboard(buttons []*ButtonWrapper) (tgbotapi.InlineKeyboardMarkup, string) {
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

func (r *Telegraminator) getModKeyboard() (tgbotapi.InlineKeyboardMarkup, string) {
	return r.multiChoiceKeyboard(r.getModButtons())
}

func (r *Telegraminator) getFnKeyboard(tcr *TelegramCallbackResponse) (tgbotapi.InlineKeyboardMarkup, string) {
	return r.multiChoiceKeyboard(r.getFnButtons(tcr))
}

func (r *Telegraminator) getArgsKeyboard(arg string, tcr *TelegramCallbackResponse) (tgbotapi.InlineKeyboardMarkup, string) {
	return r.multiChoiceKeyboard(r.getArgsButtons(arg, tcr))
}

func (r *Telegraminator) sendMessage(msg *tgbotapi.MessageConfig) {
	if r.lastMessage > 0 {
		d := tgbotapi.NewDeleteMessage(msg.ChatID, r.lastMessage)
		r.bot.Send(d)
		r.lastMessage = 0
	}
	m, err := r.bot.Send(*msg)
	if err != nil {
		log.Println(err)
		return
	}
	r.lastMessage = m.MessageID
}

func (r *Telegraminator) sendStartMessage(msg *tgbotapi.MessageConfig) {
	r.ge.DeleteTemporaryGraph(fmt.Sprint(msg.ChatID))
	msg.ReplyMarkup, _ = r.getModKeyboard()
	r.sendMessage(msg)
}

func (r *Telegraminator) Respond(chat *tgbotapi.Chat, callbackData string, inputText string) {
	telelogger := log.WithField("telegram", chat.ID)
	msg := tgbotapi.NewMessage(chat.ID, "Hello "+chat.FirstName+".")
	allowed := false
	for _, v := range r.tgc.AllowedUsers {
		if v == chat.UserName {
			allowed = true
			break
		}
	}
	if !allowed {
		msg.Text += " I'm not allowed to talk to you."
		if _, err := r.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}

	tcr, ok := r.chatState[chat.ID]
	if !ok {
		tcr = TelegramCallbackResponse{}
		if callbackData != "" {
			// a button on the InlineKeyboard was pressed
			byt := []byte(callbackData)
			err := json.Unmarshal(byt, &tcr)
			if err != nil {
				msg.Text = err.Error()
				r.sendStartMessage(&msg)
				return
			}
		} else {
			// inputText contains the mod to use
			r.ge.DeleteTemporaryGraph(fmt.Sprint(chat.ID))
			tcr.P = -1
			tcr.C = inputText
		}
	} else {
		// the user was asked to provide input
		tcr.C = inputText
		delete(r.chatState, chat.ID)
	}
	tcr.T = fmt.Sprint(chat.ID)
	op, god := r.getCurrentOp(tcr.T)
	if op == nil {
		if !r.ge.HasOperator(tcr.C) {
			msg.Text += " Please pick an Operator"
			r.sendStartMessage(&msg)
			return
		}
		tpl := &freepsgraph.GraphDesc{Operations: []freepsgraph.GraphOperationDesc{{Operator: tcr.C}}}
		r.ge.AddTemporaryGraph(tcr.T, tpl, "telegram")
		op, god = r.getCurrentOp(tcr.T)
		msg.Text = "Pick a function for " + god.Operator
		msg.ReplyMarkup, _ = r.getFnKeyboard(&tcr)
	} else if len(god.Function) == 0 {
		if tcr.K {
			msg.Text = "Type a function for " + god.Operator
			tcr.K = false
			r.chatState[chat.ID] = tcr
		} else {
			god.Function = tcr.C
		}
	}

	if len(god.Function) > 0 && !tcr.F {
		args := op.GetPossibleArgs(god.Function)
		if tcr.P == 0 {
			god.Arguments = map[string]string{}
		}
		if tcr.K {
			msg.Text = fmt.Sprintf("Type a Value for %s (%s/%s)", args[tcr.P], god.Operator, god.Function)
			tcr.K = false
			r.chatState[chat.ID] = tcr
		} else {
			if tcr.P >= 0 {
				god.Arguments[args[tcr.P]] = tcr.C
			}
			tcr.C = ""
			tcr.P++
			if tcr.P >= len(args) {
				tcr.F = true
			} else {
				addVals := ""
				msg.Text = fmt.Sprintf("Pick a Value for %s (%s/%s)", args[tcr.P], god.Operator, god.Function)
				msg.ReplyMarkup, addVals = r.getArgsKeyboard(args[tcr.P], &tcr)
				if len(addVals) > 0 {
					// do not use SendMessage, because that message gets deleted.... yeah, I need to clean this up
					r.bot.Send(tgbotapi.NewMessage(chat.ID, "More values:"+addVals+"."))
				}
			}
		}
	}

	if tcr.F {
		ctx := utils.NewContext(telelogger)
		defer ctx.MarkResponded()
		io := r.ge.ExecuteGraph(ctx, tcr.T, map[string]string{}, freepsgraph.MakeEmptyOutput())
		byt, err := io.GetBytes()
		if err != nil {
			msg.Text = fmt.Sprintf("Error when decoding output of operation: %v", err)
		} else {
			if len(io.ContentType) > 7 && io.ContentType[0:5] == "image" {
				msg.Text = "Here is a picture for you"
				m := tgbotapi.NewPhoto(chat.ID, tgbotapi.FileBytes{Name: "picture." + io.ContentType[6:], Bytes: byt})
				if _, err := r.bot.Send(m); err != nil {
					telelogger.Error(err)
				}
			} else {
				msg.Text = fmt.Sprintf("%v: %q", io.HTTPCode, byt)
			}
			r.ge.DeleteTemporaryGraph(tcr.T)
			msg.ReplyMarkup = r.getReplyKeyboard()
		}
	}
	r.sendMessage(&msg)
}

func (r *Telegraminator) MainLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := r.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			r.Respond(update.CallbackQuery.Message.Chat, update.CallbackQuery.Data, "")
			continue
		}
		if update.Message == nil { // ignore any non-Message updates
			continue
		}
		r.Respond(update.Message.Chat, "", update.Message.Text)
	}
	log.Print("Telegram Main Loop stopped")
	r.closeChan <- 1
}

func newTgbotFromConfig(cr *utils.ConfigReader) (*tgbotapi.BotAPI, *TelegramConfig, error) {
	tgc := DefaultTelegramConfig
	err := cr.ReadSectionWithDefaults("telegrambot", &tgc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	if tgc.Token == "" {
		return nil, &tgc, fmt.Errorf("No token")
	}

	bot, err := tgbotapi.NewBotAPI(tgc.Token)
	if err != nil {
		return nil, &tgc, err
	}
	bot.Debug = tgc.DebugMessages
	return bot, &tgc, nil
}

func NewTelegramBot(cr *utils.ConfigReader, ge *freepsgraph.GraphEngine, cancel context.CancelFunc) *Telegraminator {
	bot, tgc, err := newTgbotFromConfig(cr)
	t := &Telegraminator{ge: ge, bot: bot, tgc: tgc, chatState: make(map[int64]TelegramCallbackResponse), closeChan: make(chan int)}
	if err != nil {
		log.Printf("Error on Telegram registration: %v", err)
		return t
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	go t.MainLoop()
	return t
}
